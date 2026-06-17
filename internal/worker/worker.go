package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pawi1/pkgtug/internal/compress"
	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/gitops"
)

type Config struct {
	ServerURL string
	Secret    string
	Platform  string
	WorkDir   string
	Interval  time.Duration
}

type job struct {
	ID           string          `json:"job_id"`
	PackageName  string          `json:"package"`
	GitURL       string          `json:"git_url"`
	Version      string          `json:"version"`
	BuildCommand string          `json:"build_command"`
	Binaries     []config.Binary `json:"binaries"`
	Compress     string          `json:"compress,omitempty"`
}

// ErrNoJob is returned by RunOnce when the server has no pending jobs.
var ErrNoJob = fmt.Errorf("no pending jobs")

// Run polls indefinitely until ctx is cancelled.
func Run(ctx context.Context, cfg Config) {
	client := &http.Client{Timeout: 30 * time.Second}
	log.Printf("worker: polling %s (platform=%s, interval=%s)", cfg.ServerURL, cfg.Platform, cfg.Interval)
	for {
		if err := poll(ctx, client, cfg); err != nil {
			log.Printf("worker poll: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Interval):
		}
	}
}

// RunOnce polls once (retrying until waitFor elapses if no job is available yet).
// Returns ErrNoJob if the server had nothing to build after the wait window.
func RunOnce(ctx context.Context, cfg Config, waitFor time.Duration) error {
	client := &http.Client{Timeout: 30 * time.Second}

	deadline := time.Now().Add(waitFor)
	retryInterval := 10 * time.Second

	for {
		found, err := pollOnce(ctx, client, cfg)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		// No job yet — wait and retry if within window.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrNoJob
		}
		sleep := retryInterval
		if sleep > remaining {
			sleep = remaining
		}
		log.Printf("worker: no job yet, retrying in %s (%.0fs remaining)", sleep, remaining.Seconds())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
	}
}

// pollOnce does a single GET /tug/build/next. Returns (jobFound, error).
func pollOnce(ctx context.Context, client *http.Client, cfg Config) (bool, error) {
	url := cfg.ServerURL + "/tug/build/next?platform=" + cfg.Platform
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Secret)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var j job
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return false, fmt.Errorf("decode job: %w", err)
	}

	log.Printf("worker: got job %s — %s %s", j.ID, j.PackageName, j.Version)
	return true, runJob(ctx, client, cfg, &j)
}

func poll(ctx context.Context, client *http.Client, cfg Config) error {
	_, err := pollOnce(ctx, client, cfg)
	return err
}

func runJob(ctx context.Context, client *http.Client, cfg Config, j *job) error {
	cloneDir := filepath.Join(cfg.WorkDir, j.PackageName)

	log.Printf("worker [%s]: ensuring clone at %s", j.ID, cloneDir)
	if err := gitops.EnsureClone(j.GitURL, cloneDir); err != nil {
		return postError(ctx, client, cfg, j, fmt.Sprintf("git clone/fetch: %v", err))
	}

	log.Printf("worker [%s]: checkout %s", j.ID, j.Version)
	if err := gitops.Checkout(cloneDir, j.Version); err != nil {
		return postError(ctx, client, cfg, j, fmt.Sprintf("git checkout: %v", err))
	}

	log.Printf("worker [%s]: running build: %s", j.ID, j.BuildCommand)
	if err := runBuild(ctx, cloneDir, j.BuildCommand); err != nil {
		return postError(ctx, client, cfg, j, fmt.Sprintf("build failed: %v", err))
	}

	return postSuccess(ctx, client, cfg, j, cloneDir)
}

func runBuild(ctx context.Context, dir, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Strip worker credentials from the build environment so build scripts
	// and checked-out source cannot read the bearer secret.
	cmd.Env = scrubEnv(os.Environ(), "PKGTUG_SECRET", "PKGTUG_SERVER")
	return cmd.Run()
}

// scrubEnv returns a copy of env with the named variables removed.
func scrubEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		keep := true
		for _, k := range keys {
			if strings.HasPrefix(e, k+"=") {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, e)
		}
	}
	return out
}

func postSuccess(ctx context.Context, client *http.Client, cfg Config, j *job, cloneDir string) error {
	algo, err := compress.Parse(j.Compress)
	if err != nil {
		return postError(ctx, client, cfg, j, err.Error())
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("status", "ok")
	if algo != compress.None {
		mw.WriteField("compressed", string(algo))
	}

	for _, bin := range j.Binaries {
		binPath := filepath.Join(cloneDir, bin.Path)
		f, err := os.Open(binPath)
		if err != nil {
			return postError(ctx, client, cfg, j, fmt.Sprintf("open binary %s: %v", bin.Component, err))
		}
		part, err := mw.CreateFormFile(bin.Component, bin.Component)
		if err != nil {
			f.Close()
			return err
		}
		cw, err := compress.NewWriter(part, algo)
		if err != nil {
			f.Close()
			return err
		}
		h := sha256.New()
		if _, err := io.Copy(io.MultiWriter(cw, h), f); err != nil {
			f.Close()
			return err
		}
		if err := cw.Close(); err != nil {
			f.Close()
			return err
		}
		f.Close()
		log.Printf("worker [%s]: %s sha256=%s compress=%s", j.ID, bin.Component, hex.EncodeToString(h.Sum(nil)), algo)
	}
	mw.Close()

	return postResult(ctx, client, cfg, j.ID, mw.FormDataContentType(), &buf)
}

func postError(ctx context.Context, client *http.Client, cfg Config, j *job, errMsg string) error {
	log.Printf("worker [%s]: reporting failure: %s", j.ID, errMsg)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("status", "error")
	mw.WriteField("error", errMsg)
	mw.Close()
	return postResult(ctx, client, cfg, j.ID, mw.FormDataContentType(), &buf)
}

func postResult(ctx context.Context, client *http.Client, cfg Config, jobID, contentType string, body io.Reader) error {
	url := cfg.ServerURL + "/tug/build/" + jobID + "/result"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	req.Header.Set("Authorization", "Bearer "+cfg.Secret)
	req.Header.Set("Content-Type", contentType)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post result: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("post result: status %d", resp.StatusCode)
	}
	return nil
}

// platformFromUname returns a platform string like "linux-x64" from runtime info.
func PlatformFromUname() (string, error) {
	osOut, err := exec.Command("uname", "-s").Output()
	if err != nil {
		return "", err
	}
	archOut, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "", err
	}
	osName := strings.ToLower(strings.TrimSpace(string(osOut)))
	arch := strings.TrimSpace(string(archOut))
	switch arch {
	case "x86_64":
		arch = "x64"
	case "aarch64", "arm64":
		arch = "arm64"
	case "armv7l":
		arch = "arm"
	}
	return osName + "-" + arch, nil
}
