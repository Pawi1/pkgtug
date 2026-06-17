package main

import (
	"flag"
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/tui"
)

func (a *App) cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	all := fs.Bool("all", false, "update all installed packages")
	fs.Parse(args)

	if *all {
		return a.updateAll()
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug update <package/component> | --all")
	}
	return a.updateOne(fs.Arg(0))
}

func (a *App) newProgress() client.Progress {
	if tui.IsTerminal() {
		return tui.New()
	}
	return client.PlainProgress{}
}

func (a *App) updateOne(key string) error {
	p := a.newProgress()
	updated, err := client.Update(a.cfg, a.state, key, a.platform, p)
	if err != nil {
		a.tg.UpdateFailure(key, err.Error())
		return err
	}
	if updated {
		if err := a.saveState(); err != nil {
			p.Log("save state: %v", err)
		}
		a.tg.UpdateSuccess(key, a.state[key].InstalledVersion)
	}
	return nil
}

func (a *App) updateAll() error {
	var lastErr error
	p := a.newProgress()
	for key := range a.state {
		if err := a.updateOne(key); err != nil {
			p.Log("update %s: %v", key, err)
			lastErr = err
		}
	}
	return lastErr
}
