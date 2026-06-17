package main

import (
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
)

// App holds shared state for all commands.
type App struct {
	cfg       *client.Config
	statePath string
	state     client.State
	platform  string
}

func (a *App) saveState() error {
	return client.SaveState(a.statePath, a.state)
}

// Stubs — implemented in subsequent steps.

func (a *App) cmdCheck(args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdUpdate(args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdStatus(args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdRollback(args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdSearch(args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdInstall(args []string) error {
	return fmt.Errorf("not implemented yet")
}
