package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_nonexistent(t *testing.T) {
	m, err := Load("/nonexistent/path/manifest.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if m == nil || m.Binaries == nil {
		t.Fatal("expected non-nil manifest with initialised Binaries map")
	}
}

func TestLoad_valid(t *testing.T) {
	const data = `{
		"version": "1.2.3",
		"source_url": "https://github.com/example/app",
		"auth_required": true,
		"binaries": {
			"server": {
				"linux-amd64": {
					"url": "https://tug.example.com/tug/repo/app/binaries/1.2.3/linux-amd64/server",
					"sha256": "abc123",
					"size": 4096,
					"compressed": "zstd"
				}
			}
		}
	}`
	f, _ := os.CreateTemp("", "manifest-*.json")
	f.WriteString(data)
	f.Close()
	defer os.Remove(f.Name())

	m, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version)
	}
	if m.SourceURL != "https://github.com/example/app" {
		t.Errorf("SourceURL = %q", m.SourceURL)
	}
	if !m.AuthRequired {
		t.Error("AuthRequired should be true")
	}
	b := m.Binaries["server"]["linux-amd64"]
	if b.SHA256 != "abc123" {
		t.Errorf("SHA256 = %q, want abc123", b.SHA256)
	}
	if b.Size != 4096 {
		t.Errorf("Size = %d, want 4096", b.Size)
	}
	if b.Compressed != "zstd" {
		t.Errorf("Compressed = %q, want zstd", b.Compressed)
	}
}

func TestLoad_invalidJSON(t *testing.T) {
	f, _ := os.CreateTemp("", "manifest-*.json")
	f.WriteString("not json")
	f.Close()
	defer os.Remove(f.Name())

	_, err := Load(f.Name())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := &Manifest{
		Version:   "2.0.0",
		SourceURL: "https://github.com/example/app",
		Binaries: map[string]map[string]Binary{
			"server": {
				"linux-amd64": {URL: "https://example.com/bin", SHA256: "deadbeef", Size: 1024},
			},
		},
	}

	if err := WriteAtomic(path, m); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	// No leftover temp file.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after WriteAtomic")
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load after WriteAtomic: %v", err)
	}
	if got.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", got.Version)
	}
	if got.Binaries["server"]["linux-amd64"].SHA256 != "deadbeef" {
		t.Error("SHA256 mismatch after round-trip")
	}
}

func TestWriteAtomic_createsParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "manifest.json")

	m := &Manifest{Binaries: make(map[string]map[string]Binary)}
	if err := WriteAtomic(path, m); err != nil {
		t.Fatalf("WriteAtomic with nested dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}
}
