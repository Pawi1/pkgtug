package server

import (
	"log"
	"net/http"

	"github.com/pawi1/pkgtug/internal/config"
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

	version, err := s.detectVersion(pkg)
	if err != nil {
		log.Printf("fetch %s: version detection failed: %v", name, err)
	} else {
		state := s.states[name]
		if changed := state.setVersion(version); changed {
			log.Printf("fetch %s: new version detected: %s", name, version)
			s.persistState()
		} else {
			log.Printf("fetch %s: version unchanged: %s", name, version)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) detectVersion(pkg *config.Package) (string, error) {
	switch pkg.VersionSource.Type {
	case "tag":
		return gitops.LatestTag(pkg.LocalClone, pkg.VersionSource.Pattern)
	default:
		return gitops.BranchSHA(pkg.LocalClone, pkg.VersionSource.Name)
	}
}
