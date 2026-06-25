package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pawi1/pkgtug/internal/manifest"
)

// ensureSystemDeps checks each system dependency. Missing ones are resolved by
// searching the local package manager for which package provides the file,
// then asking the user interactively before installing.
func ensureSystemDeps(component string, deps []manifest.SystemDep) error {
	if len(deps) == 0 {
		return nil
	}
	pm := detectPkgManager()

	for _, dep := range deps {
		if depSatisfied(dep.File) {
			continue
		}
		label := dep.Name
		if label == "" {
			label = dep.File
		}
		fmt.Printf("\n  missing dependency for %s: %s\n", component, label)

		if pm == nil {
			fmt.Printf("  no supported package manager found; install %s manually\n", label)
			if !confirmContinue("  continue without it?") {
				return fmt.Errorf("dependency %q not satisfied", label)
			}
			continue
		}

		fmt.Printf("  searching %s for package that provides %q...\n", pm.name, dep.File)
		pkg, err := pm.findProvider(dep.File)
		if err != nil || pkg == "" {
			fmt.Printf("  could not find a package for %q\n", dep.File)
			if !confirmContinue("  continue without it?") {
				return fmt.Errorf("dependency %q not satisfied", label)
			}
			continue
		}

		fmt.Printf("  provided by: %s\n", pkg)
		if !confirmInstall(fmt.Sprintf("  install %s via %s?", pkg, pm.name)) {
			if !confirmContinue("  continue without it?") {
				return fmt.Errorf("dependency %q not satisfied", label)
			}
			continue
		}

		if err := runPrivileged(pm.installCmd(pkg)); err != nil {
			return fmt.Errorf("failed to install %q: %w", pkg, err)
		}
		if !depSatisfied(dep.File) {
			return fmt.Errorf("%q still not available after installing %s", dep.File, pkg)
		}
		fmt.Printf("  ✓ %s installed\n", pkg)
	}
	return nil
}

// depSatisfied returns true if file is an absolute path that exists, or a
// binary name that is found in PATH.
func depSatisfied(file string) bool {
	if strings.HasPrefix(file, "/") {
		_, err := os.Stat(file)
		return err == nil
	}
	// binary name — also try ldconfig for libraries
	if _, err := exec.LookPath(file); err == nil {
		return true
	}
	// shared library: check via ldconfig
	if strings.HasPrefix(file, "lib") || strings.Contains(file, ".so") {
		out, _ := exec.Command("ldconfig", "-p").Output()
		return strings.Contains(string(out), file)
	}
	return false
}

// pkgManager abstracts a system package manager.
type pkgManager struct {
	name        string
	findProvider func(file string) (string, error)
	installCmd  func(pkg string) string
}

// detectPkgManager returns the first available package manager, or nil.
func detectPkgManager() *pkgManager {
	managers := []*pkgManager{
		aptManager(),
		dnfManager(),
		apkManager(),
		zypperManager(),
		pacmanManager(),
	}
	for _, pm := range managers {
		if _, err := exec.LookPath(strings.Fields(pm.name)[0]); err == nil {
			return pm
		}
	}
	return nil
}

func aptManager() *pkgManager {
	return &pkgManager{
		name: "apt",
		findProvider: func(file string) (string, error) {
			// prefer apt-file (searches uninstalled packages)
			if _, err := exec.LookPath("apt-file"); err == nil {
				return aptFileSearch(file)
			}
			// fallback: dpkg -S (installed packages only)
			return dpkgSearch(file)
		},
		installCmd: func(pkg string) string {
			return "apt-get install -y " + pkg
		},
	}
}

func dnfManager() *pkgManager {
	return &pkgManager{
		name: "dnf",
		findProvider: func(file string) (string, error) {
			query := file
			if !strings.HasPrefix(file, "/") {
				query = "*/" + file
			}
			out, err := exec.Command("dnf", "provides", "--quiet", query).Output()
			if err != nil {
				return "", nil
			}
			// first non-empty line before " : " is "<name>-<ver>.<arch>"
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "Last metadata") {
					continue
				}
				// strip version: "openssl-3.1.4-2.fc40.x86_64" → "openssl"
				parts := strings.SplitN(line, "-", 2)
				return parts[0], nil
			}
			return "", nil
		},
		installCmd: func(pkg string) string { return "dnf install -y " + pkg },
	}
}

func apkManager() *pkgManager {
	return &pkgManager{
		name: "apk",
		findProvider: func(file string) (string, error) {
			// apk has no offline file search; search by name heuristic
			name := file
			if idx := strings.LastIndex(file, "/"); idx >= 0 {
				name = file[idx+1:]
			}
			// strip .so* suffix for libraries
			if i := strings.Index(name, ".so"); i >= 0 {
				name = name[:i]
			}
			name = strings.TrimPrefix(name, "lib")
			out, err := exec.Command("apk", "search", "--quiet", name).Output()
			if err != nil {
				return "", nil
			}
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) == 0 || lines[0] == "" {
				return "", nil
			}
			// strip version suffix: "openssl-3.1.4-r0" → "openssl"
			pkg := lines[0]
			if idx := strings.LastIndex(pkg, "-"); idx >= 0 {
				// apk package name ends before the last two "-<ver>-r<rel>"
				parts := strings.Split(pkg, "-")
				if len(parts) >= 3 {
					pkg = strings.Join(parts[:len(parts)-2], "-")
				}
			}
			return pkg, nil
		},
		installCmd: func(pkg string) string { return "apk add " + pkg },
	}
}

func zypperManager() *pkgManager {
	return &pkgManager{
		name: "zypper",
		findProvider: func(file string) (string, error) {
			out, err := exec.Command("zypper", "--quiet", "what-provides", file).Output()
			if err != nil {
				return "", nil
			}
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "S ") || strings.HasPrefix(line, "Name") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					return fields[1], nil
				}
			}
			return "", nil
		},
		installCmd: func(pkg string) string { return "zypper install -y " + pkg },
	}
}

func pacmanManager() *pkgManager {
	return &pkgManager{
		name: "pacman",
		findProvider: func(file string) (string, error) {
			// try file database first (pacman -F)
			query := file
			if !strings.HasPrefix(file, "/") {
				query = "usr/bin/" + file
			}
			out, err := exec.Command("pacman", "-F", "--quiet", query).Output()
			if err == nil && len(out) > 0 {
				// output: "<repo>/<pkg> <ver>\n    <path>"
				for _, line := range strings.Split(string(out), "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "/") {
						continue
					}
					// "<repo>/<pkg>"
					if idx := strings.Index(line, "/"); idx >= 0 {
						parts := strings.Fields(line[idx+1:])
						if len(parts) > 0 {
							return parts[0], nil
						}
					}
				}
			}
			// fallback: pkgfile if available
			if _, err := exec.LookPath("pkgfile"); err == nil {
				out, err := exec.Command("pkgfile", file).Output()
				if err == nil {
					return strings.TrimSpace(strings.Split(string(out), "\n")[0]), nil
				}
			}
			return "", nil
		},
		installCmd: func(pkg string) string { return "pacman -S --noconfirm " + pkg },
	}
}

func aptFileSearch(file string) (string, error) {
	query := file
	if !strings.HasPrefix(file, "/") {
		query = file // apt-file searches by pattern
	}
	out, err := exec.Command("apt-file", "search", "--regexp", "/("+query+")$").Output()
	if err != nil || len(out) == 0 {
		return dpkgSearch(file)
	}
	// output: "<pkg>: <path>"
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	pkg, _, ok := strings.Cut(line, ":")
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(pkg), nil
}

func dpkgSearch(file string) (string, error) {
	path := file
	if !strings.HasPrefix(file, "/") {
		if p, err := exec.LookPath(file); err == nil {
			path = p
		}
	}
	out, err := exec.Command("dpkg", "-S", path).Output()
	if err != nil {
		return "", nil
	}
	// output: "<pkg>: <path>"
	pkg, _, ok := strings.Cut(strings.TrimSpace(string(out)), ":")
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(pkg), nil
}

// runPrivileged runs cmd under sudo/doas/run0 when not root.
func runPrivileged(cmd string) error {
	if os.Getuid() == 0 {
		c := exec.Command("sh", "-c", cmd)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}
	for _, tool := range []string{"sudo", "doas", "run0"} {
		if _, err := exec.LookPath(tool); err == nil {
			c := exec.Command(tool, "sh", "-c", cmd)
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			return c.Run()
		}
	}
	return fmt.Errorf("no privilege escalation tool found (sudo/doas/run0)")
}

func runDetect(cmd string) bool {
	return exec.Command("sh", "-c", cmd+" >/dev/null 2>&1").Run() == nil
}

func confirmInstall(prompt string) bool {
	fmt.Print(prompt + " [Y/n]: ")
	line, _ := stdinReader.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "" || ans == "y"
}

func confirmContinue(prompt string) bool {
	fmt.Print(prompt + " [y/N]: ")
	line, _ := stdinReader.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y"
}
