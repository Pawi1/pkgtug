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
			ID:           randomID(),
			PackageName:  name,
			GitURL:       pkg.GitURL,
			Version:      version,
			BuildCommand: pkg.BuildCommand,
			Binaries:     pkg.Binaries,
			Platform:     platform,
			Compress:     pkg.Compress,
			ClaimedAt:    time.Now(),
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
		if err := s.storeBinary(job.PackageName, job.Version, job.Platform, bin.Component, compressed, fh); err != nil {
			fh.Close()
			return err
		}
		fh.Close()
	}
	return nil
}

// storeBinary saves one binary file and updates the package manifest atomically.
func (s *Server) storeBinary(pkgName, version, platform, component, compressed string, src io.Reader) error {
	for _, seg := range []string{pkgName, version, platform, component} {
		if err := validPathComponent(seg); err != nil {
			return fmt.Errorf("invalid upload field: %w", err)
		}
	}

	pkgRoot := filepath.Join(s.cfg.Server.DataDir, "packages", pkgName)
	pkgDir := filepath.Join(pkgRoot, version, platform)
	if err := underRoot(pkgRoot, filepath.Join(pkgDir, component)); err != nil {
		return fmt.Errorf("path traversal detected: %w", err)
	}
	// codeql[go/path-injection] - all segments validated by validPathComponent and underRoot above
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}

	destPath := filepath.Join(pkgDir, component)
	sum, err := saveFile(src, destPath)
	if err != nil {
		return fmt.Errorf("save %s: %w", component, err)
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
	}
	if mf.Binaries[component] == nil {
		mf.Binaries[component] = make(map[string]manifest.Binary)
	}
	url := fmt.Sprintf("%s/tug/repo/%s/binaries/%s/%s/%s",
		s.cfg.Server.BaseURL, pkgName, version, platform, component)
	mf.Binaries[component][platform] = manifest.Binary{URL: url, SHA256: sum, Compressed: compressed}

	return manifest.WriteAtomic(mfPath, mf)
}

// saveFile writes src to dst and returns the SHA-256 hex digest of the written bytes.
// Computing the hash during the write avoids a second read of a user-supplied path.
func saveFile(src io.Reader, dst string) (string, error) {
	// codeql[go/path-injection] - dst is derived from segments pre-validated by storeBinary callers
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(f, h), src); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
