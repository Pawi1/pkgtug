package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstallEntry holds everything pkgtug needs to manage one installed binary.
type InstallEntry struct {
	Remote           string    `json:"remote"`
	InstalledVersion string    `json:"installed_version"`
	UpdatedAt        time.Time `json:"updated_at"`
	BinaryPath       string    `json:"binary_path"`
	ServiceName      string    `json:"service_name,omitempty"`
	HealthCheck      string    `json:"health_check,omitempty"`
	BackupDir        string    `json:"backup_dir,omitempty"`
}

// State maps "<package>/<component>" → InstallEntry.
type State map[string]*InstallEntry

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(State), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

func SaveState(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}
