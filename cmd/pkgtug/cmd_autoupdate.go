package main

import (
	"flag"
	"fmt"
)

// cmdAutoupdate marks/unmarks an installed package for automatic updates by the daemon.
//
//	pkgtug autoupdate <package/component>          — enable
//	pkgtug autoupdate --remove <package/component> — disable
//	pkgtug autoupdate                              — list marked packages
func (a *App) cmdAutoupdate(args []string) error {
	fs := flag.NewFlagSet("autoupdate", flag.ExitOnError)
	remove := fs.Bool("remove", false, "remove autoupdate mark")
	fs.Parse(args)

	if fs.NArg() == 0 {
		found := false
		for key, e := range a.state {
			if e.AutoUpdate {
				fmt.Println(key)
				found = true
			}
		}
		if !found {
			fmt.Println("no packages marked for autoupdate")
		}
		return nil
	}

	key := fs.Arg(0)
	entry := a.state[key]
	if entry == nil {
		return fmt.Errorf("%s: not installed", key)
	}

	if *remove {
		entry.AutoUpdate = false
		fmt.Printf("%s: autoupdate disabled\n", key)
	} else {
		entry.AutoUpdate = true
		fmt.Printf("%s: marked for autoupdate\n", key)
	}
	return a.saveState()
}
