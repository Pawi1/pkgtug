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
	case "set-token":
		return a.remoteSetToken(args[1:])
	case "meta":
		return a.remoteMeta(args[1:])
	default:
		return fmt.Errorf("unknown remote subcommand %q", args[0])
	}
}

func (a *App) remoteAdd(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: pkgtug remote add <name> <url> [token]")
	}
	name, url := args[0], strings.TrimRight(args[1], "/")
	token := ""
	if len(args) == 3 {
		token = args[2]
	}

	if a.cfg.HasRemote(name) {
		return fmt.Errorf("remote %q already exists — remove it first", name)
	}
	a.cfg.Remotes = append(a.cfg.Remotes, client.Remote{Name: name, URL: url, Token: token})
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Printf("remote %q added (%s)\n", name, url)
	return nil
}

func (a *App) remoteSetToken(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: pkgtug remote set-token <name> [token]  (omit token to clear)")
	}
	name := args[0]
	token := ""
	if len(args) == 2 {
		token = args[1]
	}
	for i := range a.cfg.Remotes {
		if a.cfg.Remotes[i].Name == name {
			a.cfg.Remotes[i].Token = token
			if err := a.saveConfig(); err != nil {
				return err
			}
			if token == "" {
				fmt.Printf("token cleared for remote %q\n", name)
			} else {
				fmt.Printf("token set for remote %q\n", name)
			}
			return nil
		}
	}
	return fmt.Errorf("remote %q not found", name)
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
	if len(a.cfg.Remotes) == 0 && len(a.cfg.MetaURLs) == 0 {
		fmt.Println("no remotes configured")
		fmt.Println("add one with: pkgtug remote add <name> <url>")
		return nil
	}
	for _, r := range a.cfg.Remotes {
		fmt.Printf("%-20s  %s\n", r.Name, r.URL)
	}
	for _, u := range a.cfg.MetaURLs {
		fmt.Printf("%-20s  %s\n", "(meta)", u)
	}
	return nil
}

func (a *App) remoteMeta(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: pkgtug remote meta <add|list|remove> [args]")
	}
	switch args[0] {
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: pkgtug remote meta add <url>")
		}
		url := strings.TrimRight(args[1], "/")
		for _, u := range a.cfg.MetaURLs {
			if u == url {
				return fmt.Errorf("meta URL already configured: %s", url)
			}
		}
		a.cfg.MetaURLs = append(a.cfg.MetaURLs, url)
		if err := a.saveConfig(); err != nil {
			return err
		}
		fmt.Printf("meta URL added: %s\n", url)
		return nil
	case "remove", "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: pkgtug remote meta remove <url>")
		}
		url := strings.TrimRight(args[1], "/")
		before := len(a.cfg.MetaURLs)
		filtered := a.cfg.MetaURLs[:0]
		for _, u := range a.cfg.MetaURLs {
			if u != url {
				filtered = append(filtered, u)
			}
		}
		if len(filtered) == before {
			return fmt.Errorf("meta URL not found: %s", url)
		}
		a.cfg.MetaURLs = filtered
		if err := a.saveConfig(); err != nil {
			return err
		}
		fmt.Printf("meta URL removed: %s\n", url)
		return nil
	case "list", "ls":
		if len(a.cfg.MetaURLs) == 0 {
			fmt.Println("no meta URLs configured")
			fmt.Println("add one with: pkgtug remote meta add <url>")
			return nil
		}
		for _, u := range a.cfg.MetaURLs {
			fmt.Println(u)
		}
		return nil
	default:
		return fmt.Errorf("unknown meta subcommand %q", args[0])
	}
}
