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
		return fmt.Errorf("usage: pkgtug install [<remote>:]<package>[/<component>]")
	}

	remoteName, pkgName, component, err := parseInstallArg(fs.Arg(0))
	if err != nil {
		return err
	}

	// Resolve remote URL — auto-discover if not specified
	serverURL, remoteName, err := a.resolveRemote(remoteName, pkgName)
	if err != nil {
		return err
	}

	p := a.newProgress()

	p.StartSpinner("fetching manifest")
	mf, err := client.FetchManifest(serverURL, pkgName)
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

	fmt.Printf("\nInstalling %s from %s (version %s)\n\n", key, remoteName, mf.Version)

	defaultPath := filepath.Join("/opt", pkgName, component)
	binaryPath := prompt("Binary path", defaultPath)
	serviceName := promptOptional("Service name (for stop/start during update)")
	healthCheck := promptOptional("Health check URL or command")
	backupDir := promptOptional("Backup directory (for rollback)")

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

	a.state[key] = &client.InstallEntry{
		Remote:           remoteName,
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
	fmt.Printf("  remote:   %s\n", remoteName)
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

// parseInstallArg parses "[<remote>:]<package>[/<component>]".
func parseInstallArg(arg string) (remoteName, pkgName, component string, err error) {
	if idx := strings.Index(arg, ":"); idx >= 0 {
		remoteName = arg[:idx]
		arg = arg[idx+1:]
	}
	if idx := strings.Index(arg, "/"); idx >= 0 {
		pkgName = arg[:idx]
		component = arg[idx+1:]
	} else {
		pkgName = arg
	}
	if pkgName == "" {
		err = fmt.Errorf("invalid argument — expected [<remote>:]<package>[/<component>]")
	}
	return
}

// resolveRemote returns (serverURL, remoteName) for install.
// If remoteName is empty it searches all remotes for the package.
func (a *App) resolveRemote(remoteName, pkgName string) (string, string, error) {
	if remoteName != "" {
		url, err := a.cfg.RemoteURL(remoteName)
		return url, remoteName, err
	}
	if len(a.cfg.Remotes) == 0 {
		return "", "", fmt.Errorf("no remotes configured — run: pkgtug remote add <name> <url>")
	}
	if len(a.cfg.Remotes) == 1 {
		r := a.cfg.Remotes[0]
		return r.URL, r.Name, nil
	}
	// Search all remotes for the package
	p := a.newProgress()
	for _, r := range a.cfg.Remotes {
		p.StartSpinner("checking " + r.Name)
		list, err := client.FetchPackages(r.URL)
		p.StopSpinner()
		if err != nil {
			continue
		}
		for _, e := range list {
			if e.Name == pkgName {
				fmt.Printf("found %s on remote %q\n", pkgName, r.Name)
				return r.URL, r.Name, nil
			}
		}
	}
	return "", "", fmt.Errorf("package %q not found on any remote", pkgName)
}
