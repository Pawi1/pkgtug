package server

import (
	"log"
	"net/http"

	"github.com/pawi1/pkgtug/internal/gitops"
)

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	pkg, ok := s.packages[name]
	if !ok {
		http.Error(w, "package not found", http.StatusNotFound)
		return
	}

	mu := s.packageMu(name)
	if !mu.TryLock() {
		// Another fetch is already running for this package — skip silently.
		// Webhook sources (GitHub/GitLab) retry on their own schedule.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	defer mu.Unlock()

	log.Printf("fetch %s: starting git fetch", name)
	if err := gitops.Fetch(pkg.LocalClone, pkg.VersionSource.Type, pkg.VersionSource.Name); err != nil {
		log.Printf("fetch %s: %v", name, err)
		http.Error(w, "git fetch failed", http.StatusInternalServerError)
		return
	}
	log.Printf("fetch %s: done", name)
	w.WriteHeader(http.StatusNoContent)
}
