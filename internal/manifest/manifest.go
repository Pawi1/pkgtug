package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	Version      string                       `json:"version"`
	SourceURL    string                       `json:"source_url,omitempty"`    // git repo or project URL
	AuthRequired bool                         `json:"auth_required,omitempty"` // binary downloads require Authorization: Bearer token
	Binaries     map[string]map[string]Binary `json:"binaries"`                // component → platform → binary
}

type Binary struct {
	URL         string      `json:"url"`
	SHA256      string      `json:"sha256"`
	Size        int64       `json:"size"`                   // file size in bytes
	Compressed  string      `json:"compressed,omitempty"`   // "zstd" | "xz" | ""
	InstallDeps []string    `json:"install_deps,omitempty"` // components to install before this one
	Detect      string      `json:"detect,omitempty"`       // shell command; skip component if it exits non-zero
	SystemDeps  []SystemDep `json:"system_deps,omitempty"`  // system packages required before this component
}

type SystemDep struct {
	File string `json:"file"`
	Name string `json:"name,omitempty"`
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Binaries: make(map[string]map[string]Binary)}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Binaries == nil {
		m.Binaries = make(map[string]map[string]Binary)
	}
	return &m, nil
}

// WriteAtomic writes the manifest to path using a temp file + rename.
func WriteAtomic(path string, m *Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
