package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/tui"
)

// installGH handles "pkgtug install github:owner/repo[/component]".
// component, if provided, filters assets by name prefix.
func (a *App) installGH(ownerRepo, componentHint string, autoUpdate bool) error {
	p := a.newProgress()

	p.StartSpinner("fetching latest release from github.com/" + ownerRepo)
	rel, err := client.FetchLatestGHRelease(ownerRepo)
	p.StopSpinner()
	if err != nil {
		return err
	}

	fmt.Printf("\n%s — latest release: %s\n\n", ownerRepo, rel.TagName)

	installable := client.InstallableAssets(rel.Assets)
	if len(installable) == 0 {
		return fmt.Errorf("no downloadable assets in release %s", rel.TagName)
	}

	if componentHint != "" {
		var filtered []client.GHAsset
		for _, a := range installable {
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(componentHint)) {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) > 0 {
			installable = filtered
		}
	}

	idx := client.MatchGHAsset(installable, a.platform)
	if idx < 0 {
		idx = pickGHAsset(installable)
	} else {
		fmt.Printf("auto-selected: %s\n\n", installable[idx].Name)
	}
	asset := installable[idx]

	component := componentHint
	if component == "" {
		parts := strings.SplitN(ownerRepo, "/", 2)
		if len(parts) == 2 {
			component = parts[1]
		} else {
			component = ownerRepo
		}
	}
	key := strings.ReplaceAll(ownerRepo, "/", "-") + "/" + component

	defaultPath := filepath.Join("/opt", key)
	binaryPath := prompt("Binary path", defaultPath)

	fmt.Println()
	p.StartSpinner("downloading " + asset.Name)
	tmpFile, err := downloadGHAsset(asset, component, p)
	p.StopSpinner()
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	if cs := client.FindChecksumAsset(rel.Assets, asset.Name); cs != nil {
		p.StartSpinner("verifying checksum")
		err = verifyGHChecksumLocal(tmpFile, asset.Name, cs.BrowserDownloadURL)
		p.StopSpinner()
		if err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
		fmt.Println("✓ checksum verified")
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.Chmod(tmpFile, 0o755); err != nil {
		return fmt.Errorf("set permissions: %w", err)
	}
	if err := os.Rename(tmpFile, binaryPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	installedSHA, _ := client.SHA256File(binaryPath)
	a.state[key] = &client.InstallEntry{
		Remote:          "github:" + ownerRepo,
		GHSource:        ownerRepo,
		InstalledVersion: rel.TagName,
		UpdatedAt:       time.Now().UTC(),
		BinaryPath:      binaryPath,
		AutoUpdate:      autoUpdate,
		InstalledSHA256: installedSHA,
	}
	if err := a.saveState(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("\n✓ %s installed to %s\n", key, binaryPath)
	fmt.Printf("  source:   github.com/%s\n", ownerRepo)
	fmt.Printf("  version:  %s\n", rel.TagName)
	fmt.Printf("  asset:    %s\n", asset.Name)
	return nil
}

func pickGHAsset(assets []client.GHAsset) int {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = fmt.Sprintf("%-50s  (%s)", a.Name, formatBytes(a.Size))
	}
	idx, _ := pickFromList("Select asset to download:", names)
	return idx
}

func downloadGHAsset(asset client.GHAsset, name string, p client.Progress) (string, error) {
	resp, err := http.Get(asset.BrowserDownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "pkgtug-gh-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	var dst io.Writer = tmp
	if tui.IsTerminal() {
		if pw := p.DownloadWriter(name, resp.ContentLength); pw != nil {
			dst = io.MultiWriter(tmp, pw)
		}
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	p.FinishDownload()
	return tmp.Name(), nil
}

func verifyGHChecksumLocal(tmpFile, assetName, checksumURL string) error {
	resp, err := http.Get(checksumURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[1] == assetName || fields[1] == "*"+assetName) {
			return client.VerifySHA256File(tmpFile, fields[0])
		}
	}
	trimmed := strings.TrimSpace(content)
	if len(trimmed) == 64 {
		return client.VerifySHA256File(tmpFile, trimmed)
	}
	return fmt.Errorf("no checksum found for %q in checksum file", assetName)
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
