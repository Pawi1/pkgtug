package server

import (
	"os"
	"testing"
)

func TestPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()

	states := map[string]*packageState{
		"myapp": {
			currentVersion: "1.2.3",
			builtVersions:  map[string]string{"linux-x64": "1.2.3", "linux-arm64": "1.1.0"},
			activeJobs:     make(map[string]*Job),
		},
		"other": newPackageState(),
	}

	if err := savePersistedState(dir, states); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadPersistedState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded["myapp"].CurrentVersion != "1.2.3" {
		t.Errorf("current_version = %q", loaded["myapp"].CurrentVersion)
	}
	if loaded["myapp"].BuiltVersions["linux-x64"] != "1.2.3" {
		t.Errorf("built linux-x64 = %q", loaded["myapp"].BuiltVersions["linux-x64"])
	}
	if loaded["other"].CurrentVersion != "" {
		t.Errorf("other should be empty, got %q", loaded["other"].CurrentVersion)
	}
}

func TestPersistMissingFile(t *testing.T) {
	dir := t.TempDir()
	os.Remove(statePath(dir))

	s, err := loadPersistedState(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(s) != 0 {
		t.Errorf("expected empty state, got %v", s)
	}
}
