package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	remoteName := fs.String("remote", "", "search only this remote")
	fs.Parse(args)

	query := strings.Join(fs.Args(), " ")

	if len(a.cfg.Remotes) == 0 {
		return fmt.Errorf("no remotes configured — run: pkgtug remote add <name> <url>")
	}

	remotes := a.cfg.Remotes
	if *remoteName != "" {
		url, err := a.cfg.RemoteURL(*remoteName)
		if err != nil {
			return err
		}
		remotes = []client.Remote{{Name: *remoteName, URL: url}}
	}

	p := a.newProgress()
	found := false

	fmt.Printf("%-20s  %-30s  %s\n", "REMOTE", "NAME", "VERSION")
	fmt.Println(strings.Repeat("-", 65))

	for _, r := range remotes {
		p.StartSpinner("searching " + r.Name)
		list, err := client.FetchPackages(r.URL)
		p.StopSpinner()
		if err != nil {
			fmt.Printf("%-20s  (error: %v)\n", r.Name, err)
			continue
		}

		matches := client.FilterPackages(list, query)
		for _, e := range matches {
			v := e.Version
			if v == "" {
				v = "(no build yet)"
			}
			fmt.Printf("%-20s  %-30s  %s\n", r.Name, e.Name, v)
			found = true
		}
	}

	if !found {
		fmt.Println("no packages found")
	}
	return nil
}
