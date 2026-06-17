package main

import (
	"fmt"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdRemote(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pkgtug remote <add|remove|list> [args]")
	}
	switch args[0] {
	case "add":
		return a.remoteAdd(args[1:])
	case "remove", "rm":
		return a.remoteRemove(args[1:])
	case "list", "ls":
		return a.remoteList()
	default:
		return fmt.Errorf("unknown remote subcommand %q", args[0])
	}
}

func (a *App) remoteAdd(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: pkgtug remote add <name> <url>")
	}
	name, url := args[0], strings.TrimRight(args[1], "/")

	if a.cfg.HasRemote(name) {
		return fmt.Errorf("remote %q already exists — remove it first", name)
	}
	a.cfg.Remotes = append(a.cfg.Remotes, client.Remote{Name: name, URL: url})
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Printf("remote %q added (%s)\n", name, url)
	return nil
}

func (a *App) remoteRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: pkgtug remote remove <name>")
	}
	name := args[0]

	before := len(a.cfg.Remotes)
	filtered := a.cfg.Remotes[:0]
	for _, r := range a.cfg.Remotes {
		if r.Name != name {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == before {
		return fmt.Errorf("remote %q not found", name)
	}
	a.cfg.Remotes = filtered
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Printf("remote %q removed\n", name)
	return nil
}

func (a *App) remoteList() error {
	if len(a.cfg.Remotes) == 0 {
		fmt.Println("no remotes configured")
		fmt.Println("add one with: pkgtug remote add <name> <url>")
		return nil
	}
	for _, r := range a.cfg.Remotes {
		fmt.Printf("%-20s  %s\n", r.Name, r.URL)
	}
	return nil
}
