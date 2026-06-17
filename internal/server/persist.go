package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// packagePersistedState is the on-disk representation for one package.
type packagePersistedState struct {
	CurrentVersion string            `json:"current_version,omitempty"`
	BuiltVersions  map[string]string `json:"built_versions,omitempty"` // platform → version
}

// serverPersistedState is the full on-disk state file.
type serverPersistedState map[string]*packagePersistedState // package name → state

func statePath(dataDir string) string {
	return filepath.Join(dataDir, "server-state.json")
}

func loadPersistedState(dataDir string) (serverPersistedState, error) {
	path := statePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(serverPersistedState), nil
		}
		return nil, fmt.Errorf("read server state: %w", err)
	}
	var s serverPersistedState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse server state: %w", err)
	}
	return s, nil
}

func savePersistedState(dataDir string, states map[string]*packageState) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	ps := make(serverPersistedState, len(states))
	for name, st := range states {
		st.mu.Lock()
		ps[name] = &packagePersistedState{
			CurrentVersion: st.currentVersion,
			BuiltVersions:  copyStringMap(st.builtVersions),
		}
		st.mu.Unlock()
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	path := statePath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

func copyStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
