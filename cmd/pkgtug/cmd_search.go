package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/tui"
)

func (a *App) cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	fs.Parse(args)

	query := strings.Join(fs.Args(), " ")

	var p client.Progress = client.PlainProgress{}
	if tui.IsTerminal() {
		p = tui.New()
	}

	p.StartSpinner("fetching package list")
	list, err := client.FetchPackages(a.cfg.ServerURL)
	p.StopSpinner()
	if err != nil {
		return err
	}

	matches := client.FilterPackages(list, query)
	if len(matches) == 0 {
		fmt.Println("no packages found")
		return nil
	}

	fmt.Printf("%-30s  %s\n", "NAME", "VERSION")
	fmt.Println(strings.Repeat("-", 50))
	for _, e := range matches {
		v := e.Version
		if v == "" {
			v = "(no build yet)"
		}
		fmt.Printf("%-30s  %s\n", e.Name, v)
	}
	return nil
}
