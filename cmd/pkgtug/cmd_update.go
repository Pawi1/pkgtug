package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	all := fs.Bool("all", false, "update all installed packages")
	autoupdate := fs.Bool("autoupdate", false, "also update pkgtug itself (requires self_update in config)")
	fs.Parse(args)

	if *all {
		err := a.updateAll()
		if *autoupdate {
			if serr := a.selfUpdate(); serr != nil {
				a.newProgress().Log("autoupdate: %v", serr)
				if err == nil {
					err = serr
				}
			}
		}
		return err
	}
	if *autoupdate {
		return a.selfUpdate()
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug update <package/component> | --all [--autoupdate]")
	}
	return a.updateOne(fs.Arg(0))
}

func (a *App) selfUpdate() error {
	key := a.cfg.SelfUpdate
	if key == "" {
		return fmt.Errorf("self_update not configured — run: pkgtug autoupdate <remote>:<package>/<component>")
	}
	remote, pkg := splitRemoteKey(key)
	serverURL, err := a.remoteURL(remote)
	if err != nil {
		return fmt.Errorf("autoupdate: %w", err)
	}
	p := a.newProgress()
	updated, err := client.Update(serverURL, a.state, pkg, a.platform, p)
	if err != nil {
		return fmt.Errorf("autoupdate: %w", err)
	}
	if updated {
		if err := a.saveState(); err != nil {
			p.Log("save state: %v", err)
		}
	}
	return nil
}

// splitRemoteKey splits "[remote:]key" into (remote, key).
// Returns ("", key) if no remote prefix is present.
func splitRemoteKey(s string) (remote, key string) {
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

func (a *App) updateOne(key string) error {
	serverURL, err := a.serverURLForKey(key)
	if err != nil {
		return err
	}

	p := a.newProgress()
	updated, err := client.Update(serverURL, a.state, key, a.platform, p)
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
	for key, entry := range a.state {
		if entry.Pinned {
			p.Log("skip %s (pinned at %s)", key, entry.InstalledVersion)
			continue
		}
		if err := a.updateOne(key); err != nil {
			p.Log("update %s: %v", key, err)
			lastErr = err
		}
	}
	return lastErr
}
