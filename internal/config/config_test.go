package config

import (
	"os"
	"testing"
)

const validYAML = `
server:
  listen: ":9090"
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "supersecret"
telegram:
  bot_token: "123:ABC"
  chat_id: "-1001234567890"
packages:
  - name: myapp
    git_url: git@github.com:user/myapp.git
    local_clone: /data/repos/myapp
    version_source:
      type: tag
      pattern: "*-stable"
    build_command: "make build"
    binaries:
      - component: server
        path: dist/myapp
`

func TestLoadServer_valid(t *testing.T) {
	f, _ := os.CreateTemp("", "pkgtug-config-*.yaml")
	f.WriteString(validYAML)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := LoadServer(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Listen != ":9090" {
		t.Errorf("listen = %q", cfg.Server.Listen)
	}
	if len(cfg.Packages) != 1 || cfg.Packages[0].Name != "myapp" {
		t.Errorf("packages = %v", cfg.Packages)
	}
}

func TestLoadServer_defaultListen(t *testing.T) {
	const y = `
server:
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "secret"
packages:
  - name: a
    git_url: git@github.com:u/a.git
    local_clone: /data/a
    version_source:
      type: branch
      name: main
    build_command: make build
    binaries:
      - component: bin
        path: dist/a
`
	f, _ := os.CreateTemp("", "pkgtug-config-*.yaml")
	f.WriteString(y)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := LoadServer(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Listen != ":8080" {
		t.Errorf("expected default listen :8080, got %q", cfg.Server.Listen)
	}
}

func TestLoadServer_missingSecret(t *testing.T) {
	const y = `
server:
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
packages:
  - name: a
    git_url: git@github.com:u/a.git
    local_clone: /data/a
    version_source:
      type: branch
      name: main
    build_command: make build
    binaries:
      - component: bin
        path: dist/a
`
	f, _ := os.CreateTemp("", "pkgtug-config-*.yaml")
	f.WriteString(y)
	f.Close()
	defer os.Remove(f.Name())

	_, err := LoadServer(f.Name())
	if err == nil {
		t.Fatal("expected error for missing worker_secret")
	}
}

func TestLoadServer_badVersionSource(t *testing.T) {
	const y = `
server:
  base_url: "https://tug.example.com"
  data_dir: "/var/lib/pkgtug"
  worker_secret: "secret"
packages:
  - name: a
    git_url: git@github.com:u/a.git
    local_clone: /data/a
    version_source:
      type: nope
    build_command: make build
    binaries:
      - component: bin
        path: dist/a
`
	f, _ := os.CreateTemp("", "pkgtug-config-*.yaml")
	f.WriteString(y)
	f.Close()
	defer os.Remove(f.Name())

	_, err := LoadServer(f.Name())
	if err == nil {
		t.Fatal("expected error for bad version_source.type")
	}
}
