package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	pkg, ok := s.packages[name]
	if !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	if pkg.DownloadToken != "" {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != pkg.DownloadToken {
			w.Header().Set("WWW-Authenticate", `Bearer realm="pkgtug"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if version == "latest" {
		for _, seg := range []string{platform, component} {
			if err := validPathComponent(seg); err != nil {
				http.Error(w, "invalid path: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		v := s.states[name].getVersion()
		if v == "" {
			http.Error(w, "no build available yet", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tug/repo/%s/binaries/%s/%s/%s", name, v, platform, component), http.StatusFound)
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

// handleVersionList returns the list of stored versions for a package, newest first.
func (s *Server) handleVersionList(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, ok := s.packages[name]; !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	pkgDir := filepath.Join(s.cfg.Server.DataDir, "packages", name)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
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
