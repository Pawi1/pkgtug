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
}

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

func poll(ctx context.Context, client *http.Client, cfg Config) error {
	url := cfg.ServerURL + "/tug/build/next?platform=" + cfg.Platform
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Secret)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil // no pending jobs
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var j job
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return fmt.Errorf("decode job: %w", err)
	}

	log.Printf("worker: got job %s — %s %s", j.ID, j.PackageName, j.Version)
	return runJob(ctx, client, cfg, &j)
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
	return cmd.Run()
}

func postSuccess(ctx context.Context, client *http.Client, cfg Config, j *job, cloneDir string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("status", "ok")

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
		h := sha256.New()
		if _, err := io.Copy(io.MultiWriter(part, h), f); err != nil {
			f.Close()
			return err
		}
		f.Close()
		log.Printf("worker [%s]: %s sha256=%s", j.ID, bin.Component, hex.EncodeToString(h.Sum(nil)))
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
