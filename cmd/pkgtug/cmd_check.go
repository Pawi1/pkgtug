package main

import (
	"flag"
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/tui"
)

func (a *App) cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug check <package/component>")
	}
	key := fs.Arg(0)

	var p client.Progress = client.PlainProgress{}
	if tui.IsTerminal() {
		p = tui.New()
	}

	result, err := client.CheckWithProgress(a.cfg, a.state, key, a.platform, p)
	if err != nil {
		return err
	}

	if result.UpdateAvailable {
		fmt.Printf("%s: update available %s → %s\n", key, result.InstalledVersion, result.LatestVersion)
	} else {
		fmt.Printf("%s: up to date (%s)\n", key, result.LatestVersion)
	}
	return nil
}
