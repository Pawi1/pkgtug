package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Server   ServerSection   `yaml:"server"`
	Worker   WorkerSection   `yaml:"worker"`
	Telegram TelegramSection `yaml:"telegram"`
	Packages []Package       `yaml:"packages"`
}

type WorkerSection struct {
	Enabled  bool          `yaml:"enabled"`
	WorkDir  string        `yaml:"work_dir"`
	Interval time.Duration `yaml:"interval"`
}

type ServerSection struct {
	Listen          string        `yaml:"listen"`
	BaseURL         string        `yaml:"base_url"`
	DataDir         string        `yaml:"data_dir"`
	WorkerSecret    string        `yaml:"worker_secret"`
	CORSOrigins     []string      `yaml:"cors_origins"`     // e.g. ["*"] or ["https://user.github.io"]
	WebhookCooldown time.Duration `yaml:"webhook_cooldown"` // min gap between webhook fetches per package (default 10s)
	MaxUploadSize   ByteSize      `yaml:"max_upload_size"`  // max size per uploaded file; 0 = unlimited
}

type TelegramSection struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type Package struct {
	Name          string        `yaml:"name"`
	DirectPush    bool          `yaml:"direct_push"`   // skip git/build; accept binaries via POST /push only
	SourceURL     string        `yaml:"source_url"`    // project URL exposed in manifest; falls back to git_url
	DownloadToken string        `yaml:"download_token"` // if set, binary downloads require Authorization: Bearer <token>
	GitURL        string        `yaml:"git_url"`
	LocalClone    string        `yaml:"local_clone"`
	VersionSource VersionSource `yaml:"version_source"`
	BuildCommand  string        `yaml:"build_command"`
	Binaries      []Binary      `yaml:"binaries"`
	PollInterval  time.Duration `yaml:"poll_interval"` // 0 = disabled; e.g. "5m"
	Compress      string        `yaml:"compress"`       // "zstd" | "xz" | "" (none)
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
		if pkg.DirectPush {
			continue
		}
		if pkg.GitURL == "" {
			return fmt.Errorf("package %q: git_url is required (or set direct_push: true)", pkg.Name)
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
	if c.Server.WebhookCooldown <= 0 {
		c.Server.WebhookCooldown = 10 * time.Second
	}
	c.Server.BaseURL = strings.TrimRight(c.Server.BaseURL, "/")
	if c.Worker.Enabled && c.Worker.WorkDir == "" {
		c.Worker.WorkDir = "./worker-work"
	}
	if c.Worker.Enabled && c.Worker.Interval <= 0 {
		c.Worker.Interval = 30 * time.Second
	}
}
