package main

import (
	"flag"
	"fmt"
)

// cmdAutoupdate configures (or shows) the self_update key used by --autoupdate.
//
//	pkgtug autoupdate                          — show current setting
//	pkgtug autoupdate <remote>:<pkg>/<comp>   — set the key
//	pkgtug autoupdate --clear                  — clear the key
func (a *App) cmdAutoupdate(args []string) error {
	fs := flag.NewFlagSet("autoupdate", flag.ExitOnError)
	clear := fs.Bool("clear", false, "clear the self_update key")
	fs.Parse(args)

	if *clear {
		a.cfg.SelfUpdate = ""
		if err := a.saveConfig(); err != nil {
			return err
		}
		fmt.Println("autoupdate disabled")
		return nil
	}

	if fs.NArg() == 0 {
		if a.cfg.SelfUpdate == "" {
			fmt.Println("autoupdate: not configured")
		} else {
			fmt.Printf("autoupdate: %s\n", a.cfg.SelfUpdate)
		}
		return nil
	}

	key := fs.Arg(0)
	a.cfg.SelfUpdate = key
	if err := a.saveConfig(); err != nil {
		return err
	}
	fmt.Printf("autoupdate set to %s\n", key)
	return nil
}
