package main

import (
	"flag"
	"fmt"
)

func (a *App) cmdPin(args []string) error {
	fs := flag.NewFlagSet("pin", flag.ExitOnError)
	unpin := fs.Bool("unpin", false, "unpin (re-enable auto-updates)")
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug pin [--unpin] <package/component>")
	}
	key := fs.Arg(0)

	entry := a.state[key]
	if entry == nil {
		return fmt.Errorf("%s: not installed", key)
	}

	if *unpin {
		entry.Pinned = false
		fmt.Printf("%s unpinned — will be included in future updates\n", key)
	} else {
		entry.Pinned = true
		fmt.Printf("%s pinned at %s — skipped by update --all\n", key, entry.InstalledVersion)
	}

	return a.saveState()
}
