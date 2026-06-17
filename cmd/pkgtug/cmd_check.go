package main

import (
	"flag"
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug check <package/component>")
	}
	key := fs.Arg(0)

	result, err := client.Check(a.cfg, a.state, key, a.platform)
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
