package main

import (
	"fmt"
	"net/http"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/notify"
	"github.com/pawi1/pkgtug/internal/tui"
)

// App holds shared state for all commands.
type App struct {
	cfg       *client.Config
	cfgPath   string
	statePath string
	state     client.State
	platform  string
	tg        *notify.Telegram
}

func newApp(cfg *client.Config, cfgPath, statePath string, state client.State, platform string) *App {
	return &App{
		cfg:       cfg,
		cfgPath:   cfgPath,
		statePath: statePath,
		state:     state,
		platform:  platform,
		tg:        notify.NewTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID),
	}
}

func (a *App) saveState() error {
	return client.SaveState(a.statePath, a.state)
}

func (a *App) saveConfig() error {
	return client.SaveConfig(a.cfgPath, a.cfg)
}

func (a *App) newProgress() client.Progress {
	if tui.IsTerminal() {
		return tui.New()
	}
	return client.PlainProgress{}
}

// downloadWithToken performs an authenticated GET using the token of the named remote.
func (a *App) downloadWithToken(url, remoteName string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range a.cfg.Remotes {
		if r.Name == remoteName && r.Token != "" {
			req.Header.Set("Authorization", "Bearer "+r.Token)
			break
		}
	}
	return http.DefaultClient.Do(req)
}

// remoteURL returns the server URL for the named remote.
// Falls back to the first remote if name is empty and only one remote exists.
func (a *App) remoteURL(name string) (string, error) {
	if name == "" {
		if len(a.cfg.Remotes) == 0 {
			return "", fmt.Errorf("no remotes configured — run: pkgtug remote add <name> <url>")
		}
		if len(a.cfg.Remotes) == 1 {
			return a.cfg.Remotes[0].URL, nil
		}
		return "", fmt.Errorf("multiple remotes configured — specify remote as <remote>:<package>/<component>")
	}
	return a.cfg.RemoteURL(name)
}
