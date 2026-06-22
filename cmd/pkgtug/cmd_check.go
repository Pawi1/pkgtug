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

	var result *client.CheckResult
	var err error

	if entry := a.state[key]; entry != nil && entry.GHSource != "" {
		result, err = client.CheckGH(a.state, key, a.platform)
	} else {
		var remote client.Remote
		remote, err = a.remoteForKey(key)
		if err != nil {
			return err
		}
		result, err = client.CheckWithProgress(remote.URL, remote.Token, a.state, key, a.platform, a.newProgress())
	}
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
