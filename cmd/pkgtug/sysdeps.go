package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pawi1/pkgtug/internal/manifest"
)

// ensureSystemDeps checks each system dependency for a component. Missing deps
// are shown to the user with a prompt to install them automatically.
func ensureSystemDeps(component string, deps []manifest.SystemDep) error {
	for _, dep := range deps {
		if runDetect(dep.Detect) {
			continue
		}
		name := dep.Name
		if name == "" {
			name = dep.Detect
		}
		fmt.Printf("\n  missing dependency for %s: %s\n", component, name)
		fmt.Printf("  install command: %s\n", dep.Install)
		fmt.Print("  install now? [Y/n]: ")

		var ans string
		fmt.Fscan(os.Stdin, &ans)
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans != "" && ans != "y" {
			return fmt.Errorf("dependency %q required for %s — skipping install", name, component)
		}

		fmt.Printf("  running: %s\n", dep.Install)
		if err := runPrivileged(dep.Install); err != nil {
			return fmt.Errorf("failed to install %q: %w", name, err)
		}

		if !runDetect(dep.Detect) {
			return fmt.Errorf("dependency %q still not available after install", name)
		}
		fmt.Printf("  ✓ %s installed\n", name)
	}
	return nil
}

func runDetect(cmd string) bool {
	return exec.Command("sh", "-c", cmd+" >/dev/null 2>&1").Run() == nil
}

// runPrivileged runs cmd under sudo/doas/run0 when not root.
func runPrivileged(cmd string) error {
	if os.Getuid() == 0 {
		return exec.Command("sh", "-c", cmd).Run()
	}
	for _, tool := range []string{"sudo", "doas", "run0"} {
		if _, err := exec.LookPath(tool); err == nil {
			c := exec.Command(tool, "sh", "-c", cmd)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		}
	}
	return fmt.Errorf("no privilege escalation tool found (sudo/doas/run0)")
}
