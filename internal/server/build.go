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

const maxUploadSize = 512 << 20 // 512 MB

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

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
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
	pkgDir := filepath.Join(s.cfg.Server.DataDir, "packages", job.PackageName, job.Version, job.Platform)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return err
	}

	mfPath := filepath.Join(s.cfg.Server.DataDir, "packages", job.PackageName, "manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	mf.Version = job.Version

	for _, bin := range job.Binaries {
		fh, _, err := r.FormFile(bin.Component)
		if err != nil {
			return fmt.Errorf("component %q missing from upload: %w", bin.Component, err)
		}
		defer fh.Close()

		destPath := filepath.Join(pkgDir, bin.Component)
		if err := saveFile(fh, destPath); err != nil {
			return fmt.Errorf("save %s: %w", bin.Component, err)
		}

		sum, err := sha256File(destPath)
		if err != nil {
			return err
		}

		if mf.Binaries[bin.Component] == nil {
			mf.Binaries[bin.Component] = make(map[string]manifest.Binary)
		}
		url := fmt.Sprintf("%s/tug/repo/%s/binaries/%s/%s/%s",
			s.cfg.Server.BaseURL, job.PackageName, job.Version, job.Platform, bin.Component)
		mf.Binaries[bin.Component][job.Platform] = manifest.Binary{URL: url, SHA256: sum}
	}

	return manifest.WriteAtomic(mfPath, mf)
}

func saveFile(src io.Reader, dst string) error {
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, src)
	return err
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
