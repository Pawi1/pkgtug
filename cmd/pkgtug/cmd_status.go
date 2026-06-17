package main

import (
	"fmt"
	"strings"
)

func (a *App) cmdStatus(_ []string) error {
	if len(a.state) == 0 {
		fmt.Println("no packages installed")
		return nil
	}

	fmt.Printf("%-35s  %-15s  %-20s  %s\n", "PACKAGE/COMPONENT", "REMOTE", "VERSION", "BINARY")
	fmt.Println(strings.Repeat("-", 95))
	for key, e := range a.state {
		fmt.Printf("%-35s  %-15s  %-20s  %s\n", key, e.Remote, e.InstalledVersion, e.BinaryPath)
	}
	return nil
}

// serverURLForKey returns the server URL for an installed package's remote.
func (a *App) serverURLForKey(key string) (string, error) {
	entry := a.state[key]
	if entry == nil {
		// Not installed yet — use default remote resolution
		return a.remoteURL("")
	}
	if entry.Remote == "" {
		return a.remoteURL("")
	}
	return a.cfg.RemoteURL(entry.Remote)
}
