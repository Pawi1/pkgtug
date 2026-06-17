package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CheckResult is returned by Check.
type CheckResult struct {
	Key              string
	InstalledVersion string
	LatestVersion    string
	UpdateAvailable  bool
}

func Check(cfg *Config, state State, key, platform string) (*CheckResult, error) {
	return CheckWithProgress(cfg, state, key, platform, PlainProgress{})
}

func CheckWithProgress(cfg *Config, state State, key, platform string, p Progress) (*CheckResult, error) {
	pkg, component, err := splitKey(key)
	if err != nil {
		return nil, err
	}

	entry := state[key]
	installed := ""
	if entry != nil {
		installed = entry.InstalledVersion
	}

	p.StartSpinner("checking " + key)
	mf, err := FetchManifest(cfg.ServerURL, pkg)
	p.StopSpinner()
	if err != nil {
		return nil, err
	}

	binaries, ok := mf.Binaries[component]
	if !ok {
		return nil, fmt.Errorf("component %q not found in manifest", component)
	}
	if _, ok := binaries[platform]; !ok {
		return nil, fmt.Errorf("platform %q not available for %s", platform, key)
	}

	return &CheckResult{
		Key:              key,
		InstalledVersion: installed,
		LatestVersion:    mf.Version,
		UpdateAvailable:  installed != mf.Version,
	}, nil
}

// Update performs the full update flow for one installed entry.
// It returns (updated bool, err).
func Update(cfg *Config, state State, key, platform string, p Progress) (bool, error) {
	if p == nil {
		p = PlainProgress{}
	}

	result, err := CheckWithProgress(cfg, state, key, platform, p)
	if err != nil {
		return false, err
	}
	if !result.UpdateAvailable {
		p.Log("%s: already at latest (%s)", key, result.LatestVersion)
		return false, nil
	}
	p.Log("%s: updating %s → %s", key, result.InstalledVersion, result.LatestVersion)

	entry := state[key]
	if entry == nil {
		return false, fmt.Errorf("%s: not installed (run pkgtug install first)", key)
	}

	pkg, component, _ := splitKey(key)
	mf, err := FetchManifest(cfg.ServerURL, pkg)
	if err != nil {
		return false, err
	}
	bin := mf.Binaries[component][platform]

	// Download to temp file
	tmpFile, err := downloadToTemp(bin.URL, component, p)
	if err != nil {
		return false, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	// Verify SHA256
	p.StartSpinner("verifying SHA256")
	verifyErr := verifySHA256(tmpFile, bin.SHA256)
	p.StopSpinner()
	if verifyErr != nil {
		return false, fmt.Errorf("sha256 mismatch: %w", verifyErr)
	}

	// Backup current binary
	backupPath := ""
	if entry.BackupDir != "" {
		backupPath, err = backupBinary(entry.BinaryPath, entry.BackupDir, component)
		if err != nil {
			return false, fmt.Errorf("backup: %w", err)
		}
		p.Log("%s: backup → %s", key, backupPath)
	}

	// Stop service
	if entry.ServiceName != "" {
		p.Log("%s: stopping service %s", key, entry.ServiceName)
		if err := stopService(entry.ServiceName); err != nil {
			return false, fmt.Errorf("stop service: %w", err)
		}
	}

	// Atomic replace
	if err := atomicReplace(tmpFile, entry.BinaryPath); err != nil {
		startService(entry.ServiceName) // best-effort restart
		return false, fmt.Errorf("replace binary: %w", err)
	}
	p.Log("%s: binary replaced", key)

	// Start service
	if entry.ServiceName != "" {
		p.Log("%s: starting service %s", key, entry.ServiceName)
		if err := startService(entry.ServiceName); err != nil {
			doRollback(entry, backupPath, p)
			return false, fmt.Errorf("start service: %w", err)
		}
	}

	// Health check
	if entry.HealthCheck != "" {
		p.StartSpinner("health check")
		hcErr := healthCheck(entry.HealthCheck)
		p.StopSpinner()
		if hcErr != nil {
			p.Log("%s: health check failed: %v — rolling back", key, hcErr)
			doRollback(entry, backupPath, p)
			return false, fmt.Errorf("health check: %w", hcErr)
		}
	}

	// Update state
	entry.InstalledVersion = mf.Version
	entry.UpdatedAt = time.Now().UTC()
	p.Log("%s: ✓ updated to %s", key, mf.Version)
	return true, nil
}

func doRollback(entry *InstallEntry, backupPath string, p Progress) {
	if backupPath == "" {
		p.Log("rollback: no backup available")
		return
	}
	if err := atomicReplace(backupPath, entry.BinaryPath); err != nil {
		p.Log("rollback: replace failed: %v", err)
		return
	}
	if entry.ServiceName != "" {
		if err := startService(entry.ServiceName); err != nil {
			p.Log("rollback: start service failed: %v", err)
		}
	}
	p.Log("rollback: restored from %s", backupPath)
}

func downloadToTemp(url, name string, p Progress) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "pkgtug-download-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	var dst io.Writer = tmp
	if pw := p.DownloadWriter(name, resp.ContentLength); pw != nil {
		dst = io.MultiWriter(tmp, pw)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	p.FinishDownload()
	return tmp.Name(), nil
}

func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("got %s, want %s", got, expected)
	}
	return nil
}

func backupBinary(src, backupDir, component string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(backupDir, component+".bak")
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return dst, err
}

func atomicReplace(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(src, 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func healthCheck(check string) error {
	time.Sleep(2 * time.Second) // give service a moment to start
	if strings.HasPrefix(check, "http://") || strings.HasPrefix(check, "https://") {
		return healthCheckURL(check)
	}
	return healthCheckCmd(check)
}

func healthCheckURL(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	for i := range 5 {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				return nil
			}
		}
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("health check URL %s did not return 2xx after retries", url)
}

func healthCheckCmd(command string) error {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %q failed: %w\n%s", command, err, out)
	}
	return nil
}

func splitKey(key string) (pkg, component string, err error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid key %q: expected <package>/<component>", key)
	}
	return parts[0], parts[1], nil
}
