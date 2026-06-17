package main

import (
	"flag"
	"fmt"
	"os"
)

func (a *App) cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	removeBinary := fs.Bool("remove-binary", false, "also delete the binary file from disk")
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug uninstall [--remove-binary] <package/component>")
	}
	key := fs.Arg(0)

	entry := a.state[key]
	if entry == nil {
		return fmt.Errorf("%s: not installed", key)
	}

	if *removeBinary {
		if err := os.Remove(entry.BinaryPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove binary: %w", err)
		}
		fmt.Printf("removed %s\n", entry.BinaryPath)
	}

	delete(a.state, key)
	if err := a.saveState(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("uninstalled %s\n", key)
	return nil
}
