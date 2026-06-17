package main

import (
	"flag"
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
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

func (a *App) updateOne(key string) error {
	entry := a.state[key]
	p := a.newProgress()

	// GitHub source — use dedicated update path.
	if entry != nil && entry.GHSource != "" {
		updated, err := client.UpdateGH(a.state, key, a.platform, p, func(assets []client.GHAsset) int {
			return pickGHAsset(assets)
		})
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

	serverURL, err := a.serverURLForKey(key)
	if err != nil {
		return err
	}
	updated, err := updateEntry(a, serverURL, key, p)
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

// updateEntry performs a single update and returns whether the binary changed.
func updateEntry(a *App, serverURL, key string, p client.Progress) (bool, error) {
	return client.Update(serverURL, a.state, key, a.platform, p)
}

func (a *App) updateAll() error {
	order, err := topoSort(a.state)
	if err != nil {
		return fmt.Errorf("resolve dependencies: %w", err)
	}

	var lastErr error
	p := a.newProgress()
	for _, key := range order {
		entry := a.state[key]
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

// topoSort returns state keys in dependency order (deps before dependants).
// Detects cycles and returns an error if one is found.
func topoSort(state client.State) ([]string, error) {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(state))
	var order []string

	var visit func(key string) error
	visit = func(key string) error {
		if color[key] == black {
			return nil
		}
		if color[key] == gray {
			return fmt.Errorf("dependency cycle detected at %q", key)
		}
		color[key] = gray
		entry := state[key]
		if entry != nil {
			for _, dep := range entry.DependsOn {
				if _, ok := state[dep]; !ok {
					continue // dep not installed — skip silently
				}
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[key] = black
		order = append(order, key)
		return nil
	}

	for key := range state {
		if err := visit(key); err != nil {
			return nil, err
		}
	}
	return order, nil
}
