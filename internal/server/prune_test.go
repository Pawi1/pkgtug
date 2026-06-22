package server

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pawi1/pkgtug/internal/config"
)

// makeVersionDirs creates version directories under <dataDir>/packages/<pkg>/.
func makeVersionDirs(t *testing.T, dataDir, pkg string, versions []string) {
	t.Helper()
	for _, v := range versions {
		dir := filepath.Join(dataDir, "packages", pkg, v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
}

// listVersionDirs returns the version directory names present on disk.
func listVersionDirs(t *testing.T, dataDir, pkg string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "packages", pkg))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)
	return versions
}

func newTestServer(dataDir string, keepVersions int, packages ...*config.Package) *Server {
	pkgs := make(map[string]*config.Package, len(packages))
	for _, p := range packages {
		pkgs[p.Name] = p
	}
	return &Server{
		cfg: &config.ServerConfig{
			Server: config.ServerSection{
				DataDir:      dataDir,
				KeepVersions: keepVersions,
			},
		},
		packages: pkgs,
	}
}

func TestPruneOldVersions_unlimited(t *testing.T) {
	dir := t.TempDir()
	versions := []string{"v1", "v2", "v3", "v4", "v5"}
	makeVersionDirs(t, dir, "myapp", versions)

	s := newTestServer(dir, 0) // keep=0 means unlimited
	s.pruneOldVersions("myapp", "v5")

	got := listVersionDirs(t, dir, "myapp")
	if len(got) != 5 {
		t.Errorf("expected 5 versions, got %v", got)
	}
}

func TestPruneOldVersions_keepsLatest(t *testing.T) {
	dir := t.TempDir()
	versions := []string{"v1", "v2", "v3", "v4", "v5"}
	makeVersionDirs(t, dir, "myapp", versions)

	s := newTestServer(dir, 3)
	s.pruneOldVersions("myapp", "v5")

	got := listVersionDirs(t, dir, "myapp")
	if len(got) != 3 {
		t.Errorf("expected 3 versions, got %v", got)
	}
	for _, want := range []string{"v3", "v4", "v5"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected version %q to be kept, got %v", want, got)
		}
	}
}

func TestPruneOldVersions_preservesCurrent(t *testing.T) {
	dir := t.TempDir()
	// v1 is the oldest; current version is v1 (unusual but valid, e.g. rollback scenario)
	makeVersionDirs(t, dir, "myapp", []string{"v1", "v2", "v3", "v4", "v5"})

	s := newTestServer(dir, 2)
	s.pruneOldVersions("myapp", "v1") // current is the oldest

	got := listVersionDirs(t, dir, "myapp")
	// Should keep v1 (current) + 1 more = v5 (or v4, v5 depending on implementation)
	// current must always be present
	found := false
	for _, g := range got {
		if g == "v1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("current version v1 should always be kept, got %v", got)
	}
}

func TestPruneOldVersions_perPackageOverride(t *testing.T) {
	dir := t.TempDir()
	makeVersionDirs(t, dir, "myapp", []string{"v1", "v2", "v3", "v4", "v5"})

	pkg := &config.Package{Name: "myapp", KeepVersions: 2}
	s := newTestServer(dir, 10, pkg) // global=10, package overrides to 2
	s.pruneOldVersions("myapp", "v5")

	got := listVersionDirs(t, dir, "myapp")
	if len(got) != 2 {
		t.Errorf("expected 2 versions (per-package override), got %v", got)
	}
}

func TestPruneOldVersions_fewerThanKeep(t *testing.T) {
	dir := t.TempDir()
	makeVersionDirs(t, dir, "myapp", []string{"v1", "v2"})

	s := newTestServer(dir, 5)
	s.pruneOldVersions("myapp", "v2")

	got := listVersionDirs(t, dir, "myapp")
	if len(got) != 2 {
		t.Errorf("expected 2 versions (nothing to prune), got %v", got)
	}
}
