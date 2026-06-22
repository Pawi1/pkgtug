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
	"github.com/pawi1/pkgtug/internal/compress"
	"github.com/pawi1/pkgtug/internal/tui"
)

func (a *App) cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	autoUpdate := fs.Bool("autoupdate", false, "mark package for automatic updates by the daemon")
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug install [github:<owner>/<repo> | [<remote>:]<package>[/<component>]]")
	}

	// GitHub Releases shortcut: "github:owner/repo[/component-hint]"
	if strings.HasPrefix(fs.Arg(0), "github:") {
		spec := strings.TrimPrefix(fs.Arg(0), "github:")
		// spec = "owner/repo" or "owner/repo/hint"
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) < 2 {
			return fmt.Errorf("usage: pkgtug install github:<owner>/<repo>[/<component>]")
		}
		ownerRepo := parts[0] + "/" + parts[1]
		componentHint := ""
		if len(parts) == 3 {
			componentHint = parts[2]
		}
		return a.installGH(ownerRepo, componentHint, *autoUpdate)
	}

	remoteName, pkgName, component, err := parseInstallArg(fs.Arg(0))
	if err != nil {
		return err
	}

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
	postInstall := promptPostInstall(binaryPath)
	serviceName := promptServiceName()
	healthCheck := promptOptional("Health check URL or command")
	backupDir := promptOptional("Backup directory (for rollback)")
	deps := promptDependencies(a)

	algo, err := compress.Parse(bin.Compressed)
	if err != nil {
		return err
	}

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

	body := io.Reader(resp.Body)
	if algo != compress.None {
		rc, err := compress.NewReader(resp.Body, algo)
		if err != nil {
			tmp.Close()
			return fmt.Errorf("decompress: %w", err)
		}
		defer rc.Close()
		body = rc
	}

	var dst io.Writer = tmp
	size := resp.ContentLength
	if algo != compress.None {
		size = -1
	}
	if tui.IsTerminal() {
		ui := tui.New()
		if pw := ui.DownloadWriter(component, size); pw != nil {
			dst = io.MultiWriter(tmp, pw)
		}
	}
	if _, err := io.Copy(dst, body); err != nil {
		tmp.Close()
		return fmt.Errorf("download write: %w", err)
	}
	tmp.Close()
	fmt.Println()

	if bin.SHA256 != "" {
		fmt.Print("Verifying SHA256... ")
		if err := client.VerifySHA256File(tmpName, bin.SHA256); err != nil {
			return fmt.Errorf("sha256 mismatch: %w", err)
		}
		fmt.Println("OK")
	}

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpName, binaryPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	installedSHA, _ := client.SHA256File(binaryPath)
	a.state[key] = &client.InstallEntry{
		Remote:           remoteName,
		InstalledVersion: mf.Version,
		UpdatedAt:        time.Now().UTC(),
		BinaryPath:       binaryPath,
		PostInstall:      postInstall,
		ServiceName:      serviceName,
		HealthCheck:      healthCheck,
		BackupDir:        backupDir,
		AutoUpdate:       *autoUpdate,
		DependsOn:        deps,
		InstalledSHA256:  installedSHA,
	}
	if err := a.saveState(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	if *autoUpdate {
		fmt.Printf("\n✓ %s installed to %s (autoupdate enabled)\n", key, binaryPath)
	} else {
		fmt.Printf("\n✓ %s installed to %s\n", key, binaryPath)
	}
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

// promptPostInstall suggests a post-install command based on the install path and
// lets the user accept, edit in $EDITOR, or skip.
func promptPostInstall(binaryPath string) string {
	suggestion := suggestPostInstall(binaryPath)

	if suggestion == "" {
		fmt.Print("Post-install command (runs after each update, Enter to skip): ")
		line, _ := stdinReader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	fmt.Printf("Post-install command [%s]: ", suggestion)
	fmt.Println()
	fmt.Println("  1) accept suggestion")
	fmt.Println("  e) edit in $EDITOR")
	fmt.Println("  0) skip")

	for {
		raw := strings.TrimSpace(prompt("Select", "1"))
		switch raw {
		case "1", "":
			return suggestion
		case "0":
			return ""
		case "e", "E":
			val, err := editInEditor(suggestion)
			if err != nil {
				fmt.Printf("  editor error: %v\n", err)
				continue
			}
			return val
		default:
			fmt.Println("  enter 0, 1, or e")
		}
	}
}

// suggestPostInstall returns a sensible post-install command based on the target path.
func suggestPostInstall(path string) string {
	switch {
	case strings.HasPrefix(path, "/etc/systemd/") && strings.HasSuffix(path, ".service"):
		base := filepath.Base(path)
		unit := strings.TrimSuffix(base, ".service")
		return "systemctl daemon-reload && systemctl enable " + unit
	case strings.HasPrefix(path, "/etc/systemd/"):
		return "systemctl daemon-reload"
	case strings.HasPrefix(path, "/etc/init.d/"):
		base := filepath.Base(path)
		return "rc-update add " + base + " default"
	case strings.HasPrefix(path, "/etc/conf.d/") || strings.HasPrefix(path, "/etc/rc.d/"):
		return ""
	default:
		return ""
	}
}

// promptServiceName asks for a service name, offering a picker if services are detectable.
func promptServiceName() string {
	services := client.ListServices()
	if len(services) == 0 {
		return promptOptional("Service name (for stop/start during update)")
	}
	name, chosen := pickFromListOptional("Service to stop/start during update:", services)
	if !chosen {
		return promptOptional("Service name (or Enter to skip)")
	}
	return name
}

// promptDependencies asks which already-installed packages this package depends on.
func promptDependencies(a *App) []string {
	var keys []string
	for k := range a.state {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}
	return pickMultiFromList("Dependencies (other installed packages that must update first):", keys)
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
	// Probe each configured remote until the package is found.
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
