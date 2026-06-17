package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (a *App) cmdDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	interval := fs.Duration("interval", 15*time.Minute, "how often to check for updates")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.SetOutput(os.Stderr)
	log.Printf("pkgtug daemon: starting (interval=%s)", *interval)

	a.daemonTick()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("pkgtug daemon: shutting down")
			return nil
		case <-ticker.C:
			a.daemonTick()
		}
	}
}

func (a *App) daemonTick() {
	updated := 0
	for key, entry := range a.state {
		if !entry.AutoUpdate {
			continue
		}
		serverURL, err := a.serverURLForKey(key)
		if err != nil {
			log.Printf("daemon: %s: %v", key, err)
			continue
		}
		p := a.newProgress()
		prev := a.state[key].InstalledVersion
		didUpdate, err := updateEntry(a, serverURL, key, p)
		if err != nil {
			log.Printf("daemon: %s: update failed: %v", key, err)
			a.tg.UpdateFailure(key, err.Error())
			continue
		}
		if didUpdate {
			updated++
			log.Printf("daemon: %s: updated %s → %s", key, prev, a.state[key].InstalledVersion)
			a.tg.UpdateSuccess(key, a.state[key].InstalledVersion)
		}
	}

	if updated > 0 {
		if err := a.saveState(); err != nil {
			log.Printf("daemon: save state: %v", err)
		}
	}
}
