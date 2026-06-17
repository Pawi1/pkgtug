package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Legacy single-remote field — migrated to Remotes on load.
	ServerURL string `yaml:"server_url,omitempty"`

	Remotes    []Remote        `yaml:"remotes,omitempty"`
	Telegram   TelegramSection `yaml:"telegram,omitempty"`
	SelfUpdate string          `yaml:"self_update,omitempty"` // [remote:]package/component for pkgtug itself
}

type Remote struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type TelegramSection struct {
	BotToken string `yaml:"bot_token,omitempty"`
	ChatID   string `yaml:"chat_id,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read client config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse client config: %w", err)
	}
	// Migrate legacy server_url → remote named "default"
	if cfg.ServerURL != "" && len(cfg.Remotes) == 0 {
		cfg.Remotes = []Remote{{Name: "default", URL: strings.TrimRight(cfg.ServerURL, "/")}}
		cfg.ServerURL = ""
	}
	for i := range cfg.Remotes {
		cfg.Remotes[i].URL = strings.TrimRight(cfg.Remotes[i].URL, "/")
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// RemoteURL returns the URL for the named remote, or an error if not found.
func (c *Config) RemoteURL(name string) (string, error) {
	for _, r := range c.Remotes {
		if r.Name == name {
			return r.URL, nil
		}
	}
	return "", fmt.Errorf("remote %q not found", name)
}

// HasRemote reports whether a remote with the given name exists.
func (c *Config) HasRemote(name string) bool {
	for _, r := range c.Remotes {
		if r.Name == name {
			return true
		}
	}
	return false
}
