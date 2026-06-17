package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, ok := s.packages[name]; !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	mfPath := filepath.Join(s.cfg.Server.DataDir, "packages", name, "manifest.json")
	data, err := os.ReadFile(mfPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "no build available yet", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) handleBinaryDownload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	platform := r.PathValue("platform")
	component := r.PathValue("component")

	if _, ok := s.packages[name]; !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	for _, seg := range []string{name, version, platform, component} {
		if err := validPathComponent(seg); err != nil {
			http.Error(w, "invalid path: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	binPath := filepath.Join(s.cfg.Server.DataDir, "packages", name, version, platform, component)
	pkgRoot := filepath.Join(s.cfg.Server.DataDir, "packages", name)
	if err := underRoot(pkgRoot, binPath); err != nil {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, binPath)
}

// handlePackageList returns the list of tracked packages and their current version.
func (s *Server) handlePackageList(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
	}
	list := make([]entry, 0, len(s.packages))
	for name, state := range s.states {
		list = append(list, entry{Name: name, Version: state.getVersion()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
