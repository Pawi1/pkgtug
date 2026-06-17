package server

import (
	"encoding/json"
	"net/http"
	"runtime"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	packages := make(map[string]string, len(s.states))
	for name, st := range s.states {
		packages[name] = st.getVersion()
	}

	resp := struct {
		Status   string            `json:"status"`
		Packages map[string]string `json:"packages"`
		GOOS     string            `json:"goos"`
		GOARCH   string            `json:"goarch"`
	}{
		Status:   "ok",
		Packages: packages,
		GOOS:     runtime.GOOS,
		GOARCH:   runtime.GOARCH,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
