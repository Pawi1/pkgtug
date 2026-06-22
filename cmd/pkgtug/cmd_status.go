package main

import (
	"fmt"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdStatus(_ []string) error {
	if len(a.state) == 0 {
		fmt.Println("no packages installed")
		return nil
	}

	fmt.Printf("%-35s  %-15s  %-20s  %-8s  %s\n", "PACKAGE/COMPONENT", "REMOTE", "VERSION", "FLAGS", "BINARY")
	fmt.Println(strings.Repeat("-", 105))
	for key, e := range a.state {
		flags := ""
		if e.Pinned {
			flags = "pinned"
		}
		fmt.Printf("%-35s  %-15s  %-20s  %-8s  %s\n", key, e.Remote, e.InstalledVersion, flags, e.BinaryPath)
	}
	return nil
}

// serverURLForKey returns the server URL for an installed package's remote.
func (a *App) serverURLForKey(key string) (string, error) {
	r, err := a.remoteForKey(key)
	if err != nil {
		return "", err
	}
	return r.URL, nil
}

// remoteForKey returns the Remote for an installed package (URL + token).
func (a *App) remoteForKey(key string) (client.Remote, error) {
	entry := a.state[key]
	name := ""
	if entry != nil {
		name = entry.Remote
	}
	if name == "" {
		if len(a.cfg.Remotes) == 1 {
			return a.cfg.Remotes[0], nil
		}
		if len(a.cfg.Remotes) == 0 {
			return client.Remote{}, fmt.Errorf("no remotes configured")
		}
		return client.Remote{}, fmt.Errorf("multiple remotes configured — specify remote as <remote>:<package>")
	}
	for _, r := range a.cfg.Remotes {
		if r.Name == name {
			return r, nil
		}
	}
	return client.Remote{}, fmt.Errorf("remote %q not found", name)
}
