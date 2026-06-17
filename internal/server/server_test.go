package server

import (
	"path/filepath"
	"testing"

	"github.com/pawi1/pkgtug/internal/config"
)

func minimalConfig(dataDir string, pkgNames ...string) *config.ServerConfig {
	pkgs := make([]config.Package, len(pkgNames))
	for i, name := range pkgNames {
		pkgs[i] = config.Package{
			Name:          name,
			GitURL:        "git@example.com:user/" + name + ".git",
			LocalClone:    filepath.Join(dataDir, "repos", name),
			BuildCommand:  "make build",
			VersionSource: config.VersionSource{Type: "tag", Pattern: "*-stable"},
			Binaries:      []config.Binary{{Component: "bin", Path: "dist/bin"}},
		}
	}
	return &config.ServerConfig{
		Server: config.ServerSection{
			Listen:       ":0",
			BaseURL:      "http://localhost",
			DataDir:      dataDir,
			WorkerSecret: "secret",
		},
		Packages: pkgs,
	}
}

func TestNewRestoresPersistedState(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate state file as if a previous run had detected and built a version.
	existing := map[string]*packageState{
		"myapp": {
			currentVersion: "1.0.0-stable",
			builtVersions:  map[string]string{"linux-x64": "1.0.0-stable"},
			activeJobs:     make(map[string]*Job),
		},
	}
	if err := savePersistedState(dir, existing); err != nil {
		t.Fatalf("save: %v", err)
	}

	srv := New(minimalConfig(dir, "myapp"))

	if got := srv.states["myapp"].getVersion(); got != "1.0.0-stable" {
		t.Errorf("currentVersion = %q, want 1.0.0-stable", got)
	}
	srv.states["myapp"].mu.Lock()
	bv := srv.states["myapp"].builtVersions["linux-x64"]
	srv.states["myapp"].mu.Unlock()
	if bv != "1.0.0-stable" {
		t.Errorf("builtVersions[linux-x64] = %q, want 1.0.0-stable", bv)
	}
}

func TestNewFreshPackageStartsClean(t *testing.T) {
	dir := t.TempDir()

	// State file has "oldapp", but config only has "newapp".
	existing := map[string]*packageState{
		"oldapp": {
			currentVersion: "9.9.9",
			builtVersions:  map[string]string{"linux-x64": "9.9.9"},
			activeJobs:     make(map[string]*Job),
		},
	}
	if err := savePersistedState(dir, existing); err != nil {
		t.Fatalf("save: %v", err)
	}

	srv := New(minimalConfig(dir, "newapp"))

	if got := srv.states["newapp"].getVersion(); got != "" {
		t.Errorf("new package should start with empty version, got %q", got)
	}
	if _, exists := srv.states["oldapp"]; exists {
		t.Error("removed package should not appear in server states")
	}
}

func TestNewMultiplePackagesPartialState(t *testing.T) {
	dir := t.TempDir()

	// Only "alpha" has persisted state; "beta" is new.
	existing := map[string]*packageState{
		"alpha": {
			currentVersion: "2.0.0-stable",
			builtVersions:  map[string]string{"linux-x64": "2.0.0-stable"},
			activeJobs:     make(map[string]*Job),
		},
	}
	if err := savePersistedState(dir, existing); err != nil {
		t.Fatalf("save: %v", err)
	}

	srv := New(minimalConfig(dir, "alpha", "beta"))

	if got := srv.states["alpha"].getVersion(); got != "2.0.0-stable" {
		t.Errorf("alpha currentVersion = %q, want 2.0.0-stable", got)
	}
	if got := srv.states["beta"].getVersion(); got != "" {
		t.Errorf("beta should start clean, got %q", got)
	}
}
