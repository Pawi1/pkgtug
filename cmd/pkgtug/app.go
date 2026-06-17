package main

import (
	"fmt"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/notify"
	"github.com/pawi1/pkgtug/internal/tui"
)

// App holds shared state for all commands.
type App struct {
	cfg       *client.Config
	statePath string
	state     client.State
	platform  string
	tg        *notify.Telegram
}

func newApp(cfg *client.Config, statePath string, state client.State, platform string) *App {
	return &App{
		cfg:       cfg,
		statePath: statePath,
		state:     state,
		platform:  platform,
		tg:        notify.NewTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID),
	}
}

func (a *App) saveState() error {
	return client.SaveState(a.statePath, a.state)
}

func (a *App) newProgress() client.Progress {
	if tui.IsTerminal() {
		return tui.New()
	}
	return client.PlainProgress{}
}

// Stubs — implemented in subsequent steps.

func (a *App) cmdStatus(_ []string) error {
	return fmt.Errorf("not implemented yet")
}

func (a *App) cmdRollback(_ []string) error {
	return fmt.Errorf("not implemented yet")
}

