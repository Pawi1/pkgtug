package server

import (
	"log"
	"net/http"
)

// handlePush accepts a pre-built binary and stores it directly — no worker or build step needed.
// Useful for distributing AppImages, pre-compiled binaries, or artifacts built outside pkgtug.
//
// POST /tug/repo/<name>/push
//
// Multipart fields:
//   version   — version string (required)
//   platform  — e.g. linux-x64 (required)
//   component — component name matching binaries config (required)
//   file      — binary content (required)
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	if !s.authWorker(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.PathValue("name")
	if _, ok := s.packages[name]; !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	s.limitUpload(w, r)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	version := r.FormValue("version")
	platform := r.FormValue("platform")
	component := r.FormValue("component")
	compressed := r.FormValue("compressed") // optional: "zstd" | "xz"

	switch {
	case version == "":
		http.Error(w, "version field required", http.StatusBadRequest)
		return
	case platform == "":
		http.Error(w, "platform field required", http.StatusBadRequest)
		return
	case component == "":
		http.Error(w, "component field required", http.StatusBadRequest)
		return
	}

	fh, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer fh.Close()

	if err := s.storeBinary(name, version, platform, component, compressed, fh); err != nil {
		log.Printf("push %s [%s %s %s]: store failed: %v", name, version, platform, component, err)
		http.Error(w, "store failed", http.StatusInternalServerError)
		return
	}

	// Update in-memory version and persist.
	s.states[name].setVersion(version)
	s.states[name].completeJob(platform, version)
	s.persistState()

	log.Printf("push %s [%s %s %s]: OK", name, version, platform, component)

	if err := s.tg.BuildSuccess(name, version, platform); err != nil {
		log.Printf("telegram notify: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
