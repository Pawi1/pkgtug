package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pawi1/pkgtug/internal/manifest"
)

func (s *Server) handleBuildNext(w http.ResponseWriter, r *http.Request) {
	if !s.authWorker(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		http.Error(w, "platform query param required", http.StatusBadRequest)
		return
	}

	for name, pkg := range s.packages {
		if pkg.DirectPush {
			continue
		}
		state := s.states[name]
		if !state.needsJob(platform, jobTimeout) {
			continue
		}
		version := state.getVersion()
		if version == "" {
			continue
		}
		j := &Job{
			ID:              randomID(),
			PackageName:     name,
			GitURL:          pkg.GitURL,
			Version:         version,
			PreBuildCommand: pkg.PreBuildCommand,
			BuildCommand:    pkg.BuildCommand,
			Binaries:        pkg.Binaries,
			Platform:        platform,
			Compress:        pkg.Compress,
			ClaimedAt:       time.Now(),
		}
		state.claimJob(platform, j)
		s.jobs.add(j)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(j)
		return
	}

	w.WriteHeader(http.StatusNoContent) // no pending jobs
}

func (s *Server) handleBuildResult(w http.ResponseWriter, r *http.Request) {
	if !s.authWorker(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	jobID := r.PathValue("job_id")
	job, ok := s.jobs.get(jobID)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	s.limitUpload(w, r)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	switch status {
	case "error":
		errMsg := r.FormValue("error")
		log.Printf("build result %s [%s %s %s]: FAILED: %s", jobID, job.PackageName, job.Version, job.Platform, errMsg)
		s.states[job.PackageName].failJob(job.Platform)
		s.jobs.delete(jobID)
		if err := s.tg.BuildFailure(job.PackageName, job.Version, job.Platform, errMsg); err != nil {
			log.Printf("telegram notify: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case "ok":
		if err := s.storeBuildResult(job, r); err != nil {
			log.Printf("build result %s: store failed: %v", jobID, err)
			http.Error(w, "store failed", http.StatusInternalServerError)
			return
		}
		s.states[job.PackageName].completeJob(job.Platform, job.Version)
		s.jobs.delete(jobID)
		s.persistState()
		log.Printf("build result %s [%s %s %s]: OK", jobID, job.PackageName, job.Version, job.Platform)
		if err := s.tg.BuildSuccess(job.PackageName, job.Version, job.Platform); err != nil {
			log.Printf("telegram notify: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "status must be ok or error", http.StatusBadRequest)
	}
}

func (s *Server) storeBuildResult(job *Job, r *http.Request) error {
	compressed := r.FormValue("compressed") // "zstd" | "xz" | ""
	for _, bin := range job.Binaries {
		fh, _, err := r.FormFile(bin.Component)
		if err != nil {
			return fmt.Errorf("component %q missing from upload: %w", bin.Component, err)
		}
		origSHA := r.FormValue(bin.Component + "_sha256")
		if err := s.storeBinary(job.PackageName, job.Version, job.Platform, bin.Component, compressed, origSHA, fh); err != nil {
			fh.Close()
			return err
		}
		fh.Close()
	}
	return nil
}

// storeBinary saves one binary file and updates the package manifest atomically.
// origSHA is the SHA256 of the uncompressed binary (sent by the worker); if empty
// (direct push, no compression), the SHA is computed from the stored file instead.
func (s *Server) storeBinary(pkgName, version, platform, component, compressed, origSHA string, src io.Reader) error {
	for _, seg := range []string{pkgName, version, platform, component} {
		if err := validPathComponent(seg); err != nil {
			return fmt.Errorf("invalid upload field: %w", err)
		}
	}

	// Resolve the destination to an absolute path and verify it stays within the
	// packages directory. filepath.Abs output (not the raw Join) reaches the sink,
	// which is the pattern path-injection analysers model as safe.
	absPackagesDir, err := filepath.Abs(filepath.Join(s.cfg.Server.DataDir, "packages"))
	if err != nil {
		return fmt.Errorf("resolve packages dir: %w", err)
	}
	safeRoot := absPackagesDir + string(filepath.Separator)
	destPath, err := filepath.Abs(filepath.Join(s.cfg.Server.DataDir, "packages", pkgName, version, platform, component))
	if err != nil {
		return fmt.Errorf("resolve dest path: %w", err)
	}
	if !strings.HasPrefix(destPath, safeRoot) {
		return fmt.Errorf("path traversal detected")
	}

	pkgDir := filepath.Dir(destPath)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}

	storedSum, size, err := saveFile(src, destPath)
	if err != nil {
		return fmt.Errorf("save %s: %w", component, err)
	}
	// Use the pre-compression SHA when available (worker builds with compression).
	// The client decompresses before verifying, so the manifest must hold the
	// uncompressed SHA. For direct pushes the file arrives as-is, so fall back
	// to the stored-file SHA.
	manifestSHA := origSHA
	if manifestSHA == "" {
		manifestSHA = storedSum
	}

	mfPath := filepath.Join(s.cfg.Server.DataDir, "packages", pkgName, "manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	mf.Version = version
	if pkg, ok := s.packages[pkgName]; ok {
		switch {
		case pkg.SourceURL != "":
			mf.SourceURL = pkg.SourceURL
		case pkg.GitURL != "":
			mf.SourceURL = pkg.GitURL
		}
		mf.AuthRequired = pkg.DownloadToken != ""
	}
	if mf.Binaries[component] == nil {
		mf.Binaries[component] = make(map[string]manifest.Binary)
	}
	url := fmt.Sprintf("%s/tug/repo/%s/binaries/%s/%s/%s",
		s.cfg.Server.BaseURL, pkgName, version, platform, component)
	entry := manifest.Binary{URL: url, SHA256: manifestSHA, Size: size, Compressed: compressed}
	if pkg, ok := s.packages[pkgName]; ok {
		for _, b := range pkg.Binaries {
			if b.Component == component {
				entry.InstallDeps = b.InstallDeps
				entry.Detect = b.Detect
				break
			}
		}
	}
	mf.Binaries[component][platform] = entry

	if err := manifest.WriteAtomic(mfPath, mf); err != nil {
		return err
	}

	s.pruneOldVersions(pkgName, version)
	return nil
}

// pruneOldVersions removes old version directories beyond the configured keep limit.
// The current version is always kept. Directories are sorted lexicographically and
// the oldest (lowest sort order) are removed first.
func (s *Server) pruneOldVersions(pkgName, currentVersion string) {
	keep := s.cfg.Server.KeepVersions
	if pkg, ok := s.packages[pkgName]; ok && pkg.KeepVersions > 0 {
		keep = pkg.KeepVersions
	}
	if keep <= 0 {
		return // unlimited
	}

	pkgRoot := filepath.Join(s.cfg.Server.DataDir, "packages", pkgName)
	entries, err := os.ReadDir(pkgRoot)
	if err != nil {
		return
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	// versions are sorted ascending by os.ReadDir; remove from the front (oldest first)
	// always keep currentVersion regardless of position
	toDelete := len(versions) - keep
	deleted := 0
	for _, v := range versions {
		if deleted >= toDelete {
			break
		}
		if v == currentVersion {
			continue
		}
		dir := filepath.Join(pkgRoot, v)
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("prune %s/%s: %v", pkgName, v, err)
		} else {
			log.Printf("prune %s/%s: removed (keep_versions=%d)", pkgName, v, keep)
			deleted++
		}
	}
}

// saveFile writes src to dst and returns the SHA-256 hex digest and byte count of the written data.
// Computing the hash during the write avoids a second read of a user-supplied path.
func saveFile(src io.Reader, dst string) (string, int64, error) {
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), src)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
