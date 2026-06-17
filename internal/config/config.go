package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Server   ServerSection   `yaml:"server"`
	Telegram TelegramSection `yaml:"telegram"`
	Packages []Package       `yaml:"packages"`
}

type ServerSection struct {
	Listen       string `yaml:"listen"`
	BaseURL      string `yaml:"base_url"`
	DataDir      string `yaml:"data_dir"`
	WorkerSecret string `yaml:"worker_secret"`
}

type TelegramSection struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type Package struct {
	Name          string        `yaml:"name"`
	GitURL        string        `yaml:"git_url"`
	LocalClone    string        `yaml:"local_clone"`
	VersionSource VersionSource `yaml:"version_source"`
	BuildCommand  string        `yaml:"build_command"`
	Binaries      []Binary      `yaml:"binaries"`
}

type VersionSource struct {
	Type    string `yaml:"type"`    // "tag" or "branch"
	Pattern string `yaml:"pattern"` // glob, only for type=tag
	Name    string `yaml:"name"`    // branch name, only for type=branch
}

type Binary struct {
	Component string `yaml:"component"`
	Path      string `yaml:"path"`
}

func LoadServer(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *ServerConfig) validate() error {
	if c.Server.WorkerSecret == "" {
		return fmt.Errorf("server.worker_secret is required")
	}
	if c.Server.BaseURL == "" {
		return fmt.Errorf("server.base_url is required")
	}
	if c.Server.DataDir == "" {
		return fmt.Errorf("server.data_dir is required")
	}
	if len(c.Packages) == 0 {
		return fmt.Errorf("no packages defined")
	}

	seen := map[string]bool{}
	for i, pkg := range c.Packages {
		if pkg.Name == "" {
			return fmt.Errorf("packages[%d]: name is required", i)
		}
		if seen[pkg.Name] {
			return fmt.Errorf("packages[%d]: duplicate name %q", i, pkg.Name)
		}
		seen[pkg.Name] = true
		if pkg.GitURL == "" {
			return fmt.Errorf("package %q: git_url is required", pkg.Name)
		}
		if pkg.LocalClone == "" {
			return fmt.Errorf("package %q: local_clone is required", pkg.Name)
		}
		if pkg.BuildCommand == "" {
			return fmt.Errorf("package %q: build_command is required", pkg.Name)
		}
		if len(pkg.Binaries) == 0 {
			return fmt.Errorf("package %q: at least one binary is required", pkg.Name)
		}
		vs := pkg.VersionSource
		switch vs.Type {
		case "tag":
			if vs.Pattern == "" {
				return fmt.Errorf("package %q: version_source.pattern is required for type=tag", pkg.Name)
			}
		case "branch":
			if vs.Name == "" {
				return fmt.Errorf("package %q: version_source.name is required for type=branch", pkg.Name)
			}
		default:
			return fmt.Errorf("package %q: version_source.type must be \"tag\" or \"branch\", got %q", pkg.Name, vs.Type)
		}
		for j, bin := range pkg.Binaries {
			if bin.Component == "" {
				return fmt.Errorf("package %q: binaries[%d]: component is required", pkg.Name, j)
			}
			if bin.Path == "" {
				return fmt.Errorf("package %q: binaries[%d]: path is required", pkg.Name, j)
			}
		}
	}
	return nil
}

func (c *ServerConfig) applyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	c.Server.BaseURL = strings.TrimRight(c.Server.BaseURL, "/")
}
