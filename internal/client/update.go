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

	"github.com/pawi1/pkgtug/internal/compress"
)

// CheckResult is returned by Check.
type CheckResult struct {
	Key              string
	InstalledVersion string
	LatestVersion    string
	UpdateAvailable  bool
}

func Check(serverURL string, state State, key, platform string) (*CheckResult, error) {
	return CheckWithProgress(serverURL, "", state, key, platform, PlainProgress{})
}

func CheckWithProgress(serverURL, token string, state State, key, platform string, p Progress) (*CheckResult, error) {
	pkg, component, err := SplitKey(key)
	if err != nil {
		return nil, err
	}

	entry := state[key]
	installed := ""
	if entry != nil {
		installed = entry.InstalledVersion
	}

	p.StartSpinner("checking " + key)
	mf, err := FetchManifest(serverURL, pkg)
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
func Update(serverURL, token string, state State, key, platform string, p Progress) (bool, error) {
	if p == nil {
		p = PlainProgress{}
	}

	result, err := CheckWithProgress(serverURL, token, state, key, platform, p)
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

	pkg, component, _ := SplitKey(key)
	mf, err := FetchManifest(serverURL, pkg)
	if err != nil {
		return false, err
	}
	bin := mf.Binaries[component][platform]

	algo, err := compress.Parse(bin.Compressed)
	if err != nil {
		return false, err
	}
	tmpFile, err := downloadToTempCompressed(bin.URL, token, component, algo, p)
	if err != nil {
		return false, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	p.StartSpinner("verifying SHA256")
	verifyErr := verifySHA256(tmpFile, bin.SHA256)
	p.StopSpinner()
	if verifyErr != nil {
		return false, fmt.Errorf("sha256 mismatch: %w", verifyErr)
	}

	backupPath := ""
	if entry.BackupDir != "" {
		backupPath, err = backupBinary(entry.BinaryPath, entry.BackupDir, component)
		if err != nil {
			return false, fmt.Errorf("backup: %w", err)
		}
		p.Log("%s: backup → %s", key, backupPath)
	}

	if entry.ServiceName != "" {
		p.Log("%s: stopping service %s", key, entry.ServiceName)
		if err := StopService(entry.ServiceName); err != nil {
			return false, fmt.Errorf("stop service: %w", err)
		}
	}

	// Check for local edits before replacing user-editable files.
	if entry.InstalledSHA256 != "" {
		if abort, err := handleConflict(p, key, entry, tmpFile, bin.SHA256); err != nil {
			return false, err
		} else if abort {
			return false, nil
		}
	}

	if err := atomicReplace(tmpFile, entry.BinaryPath); err != nil {
		StartService(entry.ServiceName) // best-effort restart
		return false, fmt.Errorf("replace binary: %w", err)
	}
	p.Log("%s: binary replaced", key)

	if entry.PostInstall != "" {
		p.Log("%s: running post-install: %s", key, entry.PostInstall)
		cmd := exec.Command("sh", "-c", entry.PostInstall)
		if out, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("post-install: %w\n%s", err, out)
		}
	}

	if entry.ServiceName != "" {
		p.Log("%s: starting service %s", key, entry.ServiceName)
		if err := StartService(entry.ServiceName); err != nil {
			doRollback(entry, backupPath, p)
			return false, fmt.Errorf("start service: %w", err)
		}
	}

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

	entry.InstalledVersion = mf.Version
	entry.InstalledSHA256 = bin.SHA256
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
		if err := StartService(entry.ServiceName); err != nil {
			p.Log("rollback: start service failed: %v", err)
		}
	}
	p.Log("rollback: restored from %s", backupPath)
}

func downloadToTemp(url, name string, p Progress) (string, error) {
	return downloadToTempCompressed(url, "", name, compress.None, p)
}

func downloadToTempCompressed(url, token, name string, algo compress.Algo, p Progress) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
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

	body := resp.Body
	if algo != compress.None {
		rc, err := compress.NewReader(resp.Body, algo)
		if err != nil {
			os.Remove(tmp.Name())
			return "", fmt.Errorf("decompress: %w", err)
		}
		defer rc.Close()
		body = rc
	}

	var dst io.Writer = tmp
	size := resp.ContentLength
	if algo != compress.None {
		size = -1 // unknown after decompression
	}
	if pw := p.DownloadWriter(name, size); pw != nil {
		dst = io.MultiWriter(tmp, pw)
	}
	if _, err := io.Copy(dst, body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	p.FinishDownload()
	return tmp.Name(), nil
}

func VerifySHA256File(path, expected string) error { return verifySHA256(path, expected) }

func verifySHA256(path, expected string) error {
	if expected == "" {
		return nil
	}
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

// handleConflict checks whether the on-disk file was user-modified while a new
// version is also available. Returns (abort bool, err).
func handleConflict(p Progress, key string, entry *InstallEntry, newTmp, newSHA256 string) (bool, error) {
	currentSHA, err := sha256File(entry.BinaryPath)
	if err != nil {
		// File missing or unreadable — no conflict, proceed normally.
		return false, nil
	}

	userModified := currentSHA != entry.InstalledSHA256
	newDiffers := newSHA256 != entry.InstalledSHA256

	if !userModified || !newDiffers {
		return false, nil
	}

	// Both sides changed since the last installed baseline.
	diffText := runDiff(entry.BinaryPath, newTmp)
	action := p.ResolveConflict(key, entry.BinaryPath, diffText)

	switch action {
	case ConflictUseNew:
		return false, nil

	case ConflictKeepCurrent:
		dest := entry.BinaryPath + ".pkgtug-new"
		if err := copyFile(newTmp, dest); err != nil {
			p.Log("%s: could not save .pkgtug-new: %v", key, err)
		} else {
			p.Log("%s: new version saved to %s", key, dest)
		}
		return true, nil // skip the atomic replace

	case ConflictEdit:
		dest := entry.BinaryPath + ".pkgtug-new"
		_ = copyFile(newTmp, dest)
		p.Log("%s: new version at %s — edit %s, then re-run update", key, dest, entry.BinaryPath)
		return true, nil

	case ConflictAbort:
		return true, nil

	default:
		return false, nil
	}
}

func SHA256File(path string) (string, error) {
	return sha256File(path)
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

func runDiff(current, next string) string {
	out, err := exec.Command("diff", "-u", "--label", "current", "--label", "new", current, next).CombinedOutput()
	if err == nil || len(out) > 0 {
		return string(out)
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// CheckGH checks for a new version of a GitHub-sourced entry.
func CheckGH(state State, key, platform string) (*CheckResult, error) {
	entry := state[key]
	if entry == nil {
		return nil, fmt.Errorf("%s: not installed", key)
	}
	rel, err := FetchLatestGHRelease(entry.GHSource)
	if err != nil {
		return nil, err
	}
	return &CheckResult{
		Key:              key,
		InstalledVersion: entry.InstalledVersion,
		LatestVersion:    rel.TagName,
		UpdateAvailable:  entry.InstalledVersion != rel.TagName,
	}, nil
}

// UpdateGH updates a GitHub-sourced entry to the latest release.
// It always picks the same asset index by re-matching; if ambiguous it uses the picker.
// In non-TTY contexts pickFn should return -1 to abort rather than block.
func UpdateGH(state State, key, platform string, p Progress, pickFn func([]GHAsset) int) (bool, error) {
	entry := state[key]
	if entry == nil {
		return false, fmt.Errorf("%s: not installed", key)
	}

	rel, err := FetchLatestGHRelease(entry.GHSource)
	if err != nil {
		return false, err
	}
	if entry.InstalledVersion == rel.TagName {
		p.Log("%s: already at latest (%s)", key, rel.TagName)
		return false, nil
	}
	p.Log("%s: updating %s → %s", key, entry.InstalledVersion, rel.TagName)

	installable := InstallableAssets(rel.Assets)
	idx := MatchGHAsset(installable, platform)
	if idx < 0 {
		idx = pickFn(installable)
		if idx < 0 {
			return false, fmt.Errorf("%s: could not determine asset — run pkgtug install github:%s to re-select", key, entry.GHSource)
		}
	}
	asset := installable[idx]

	tmpFile, err := downloadToTemp(asset.BrowserDownloadURL, asset.Name, p)
	if err != nil {
		return false, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	// Try checksum verification if a companion file is available.
	if cs := FindChecksumAsset(rel.Assets, asset.Name); cs != nil {
		if err := verifyGHChecksum(tmpFile, asset.Name, cs.BrowserDownloadURL); err != nil {
			return false, fmt.Errorf("checksum: %w", err)
		}
	}

	newSHA, _ := sha256File(tmpFile)
	if abort, err := handleConflict(p, key, entry, tmpFile, newSHA); err != nil {
		return false, err
	} else if abort {
		return false, nil
	}

	var backupPath string
	if entry.BackupDir != "" {
		_, component, _ := SplitKey(key)
		var backupErr error
		backupPath, backupErr = backupBinary(entry.BinaryPath, entry.BackupDir, component)
		if backupErr != nil {
			return false, fmt.Errorf("backup: %w", backupErr)
		}
	}

	if entry.ServiceName != "" {
		if err := StopService(entry.ServiceName); err != nil {
			return false, fmt.Errorf("stop service: %w", err)
		}
	}

	if err := atomicReplace(tmpFile, entry.BinaryPath); err != nil {
		StartService(entry.ServiceName)
		return false, fmt.Errorf("replace binary: %w", err)
	}

	if entry.PostInstall != "" {
		cmd := exec.Command("sh", "-c", entry.PostInstall)
		if out, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("post-install: %w\n%s", err, out)
		}
	}

	if entry.ServiceName != "" {
		if err := StartService(entry.ServiceName); err != nil {
			doRollback(entry, backupPath, p)
			return false, fmt.Errorf("start service: %w", err)
		}
	}

	if entry.HealthCheck != "" {
		if err := healthCheck(entry.HealthCheck); err != nil {
			doRollback(entry, backupPath, p)
			return false, fmt.Errorf("health check: %w", err)
		}
	}

	currentSHA, _ := sha256File(entry.BinaryPath)
	entry.InstalledVersion = rel.TagName
	entry.InstalledSHA256 = currentSHA
	entry.UpdatedAt = time.Now().UTC()
	p.Log("%s: ✓ updated to %s", key, rel.TagName)
	return true, nil
}

func verifyGHChecksum(tmpFile, assetName, checksumURL string) error {
	resp, err := httpClient.Get(checksumURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	content := string(data)
	// Format: "<sha256>  <filename>" (one entry per line)
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			return verifySHA256(tmpFile, fields[0])
		}
	}
	// Single-file checksum: just the hash
	trimmed := strings.TrimSpace(content)
	if len(trimmed) == 64 { // SHA256 hex
		return verifySHA256(tmpFile, trimmed)
	}
	return nil // checksum file format unrecognised — skip
}

func SplitKey(key string) (pkg, component string, err error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid key %q: expected <package>/<component>", key)
	}
	return parts[0], parts[1], nil
}
