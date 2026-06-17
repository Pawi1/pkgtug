package main

import (
	"flag"
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

func (a *App) cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug install <package>[/<component>]")
	}

	arg := fs.Arg(0)
	var pkgName, component string

	if idx := strings.Index(arg, "/"); idx >= 0 {
		pkgName = arg[:idx]
		component = arg[idx+1:]
	} else {
		pkgName = arg
	}

	p := a.newProgress()

	// Fetch manifest to know available components
	p.StartSpinner("fetching manifest")
	mf, err := client.FetchManifest(a.cfg.ServerURL, pkgName)
	p.StopSpinner()
	if err != nil {
		return err
	}

	if len(mf.Binaries) == 0 {
		return fmt.Errorf("no binaries available for %s yet", pkgName)
	}

	// Resolve component
	if component == "" {
		components := make([]string, 0, len(mf.Binaries))
		for c := range mf.Binaries {
			components = append(components, c)
		}
		if len(components) == 1 {
			component = components[0]
		} else {
			idx, err := pickFromList("Available components:", components)
			if err != nil {
				return err
			}
			component = components[idx]
		}
	}

	bins, ok := mf.Binaries[component]
	if !ok {
		return fmt.Errorf("component %q not found in manifest for %s", component, pkgName)
	}
	bin, ok := bins[a.platform]
	if !ok {
		return fmt.Errorf("platform %q not available for %s/%s", a.platform, pkgName, component)
	}

	key := pkgName + "/" + component

	// Interactive prompts
	fmt.Printf("\nInstalling %s (version %s)\n\n", key, mf.Version)

	defaultPath := filepath.Join("/opt", pkgName, component)
	binaryPath := prompt("Binary path", defaultPath)
	serviceName := promptOptional("Service name (for stop/start during update)")
	healthCheck := promptOptional("Health check URL or command")
	backupDir := promptOptional("Backup directory (for rollback)")

	// Download
	fmt.Println()
	resp, err := http.Get(bin.URL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(binaryPath), ".pkgtug-install-*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	var dst io.Writer = tmp
	if tui.IsTerminal() {
		ui := tui.New()
		if pw := ui.DownloadWriter(component, resp.ContentLength); pw != nil {
			dst = io.MultiWriter(tmp, pw)
		}
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("download write: %w", err)
	}
	tmp.Close()
	fmt.Println()

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpName, binaryPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	// Save to state
	a.state[key] = &client.InstallEntry{
		InstalledVersion: mf.Version,
		UpdatedAt:        time.Now().UTC(),
		BinaryPath:       binaryPath,
		ServiceName:      serviceName,
		HealthCheck:      healthCheck,
		BackupDir:        backupDir,
	}
	if err := a.saveState(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("\n✓ %s installed to %s\n", key, binaryPath)
	fmt.Printf("  version:  %s\n", mf.Version)
	if serviceName != "" {
		fmt.Printf("  service:  %s\n", serviceName)
	}
	if healthCheck != "" {
		fmt.Printf("  health:   %s\n", healthCheck)
	}
	if backupDir != "" {
		fmt.Printf("  backup:   %s\n", backupDir)
	}
	return nil
}
