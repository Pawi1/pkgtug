package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *jsonOut {
		return a.cmdStatusJSON()
	}

	if len(a.state) == 0 {
		fmt.Println("no packages installed")
		return nil
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(a.state))
	for k := range a.state {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("%-35s  %-15s  %-20s  %-8s  %s\n", "PACKAGE/COMPONENT", "REMOTE", "VERSION", "FLAGS", "BINARY")
	fmt.Println(strings.Repeat("-", 105))
	for _, key := range keys {
		e := a.state[key]
		flags := ""
		if e.Pinned {
			flags = "pinned"
		}
		fmt.Printf("%-35s  %-15s  %-20s  %-8s  %s\n", key, e.Remote, e.InstalledVersion, flags, e.BinaryPath)
	}
	return nil
}

type statusEntry struct {
	Key     string `json:"key"`
	*client.InstallEntry
}

func (a *App) cmdStatusJSON() error {
	keys := make([]string, 0, len(a.state))
	for k := range a.state {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]statusEntry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, statusEntry{Key: k, InstallEntry: a.state[k]})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
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
