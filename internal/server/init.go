package server

import (
	"context"
	"log"
	"sync"

	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/gitops"
)

// Init clones any missing git repos and detects the current version for each
// package. It runs all packages concurrently and logs errors without failing —
// a missing or broken repo should not prevent the server from starting.
func (s *Server) Init(ctx context.Context) {
	var wg sync.WaitGroup
	for name, pkg := range s.packages {
		wg.Add(1)
		go func(name string, pkg *config.Package) {
			defer wg.Done()
			s.initPackage(ctx, name, pkg)
		}(name, pkg)
	}
	wg.Wait()
}

func (s *Server) initPackage(ctx context.Context, name string, pkg *config.Package) {
	if pkg.DirectPush {
		log.Printf("init %s: direct_push — skipping git clone", name)
		return
	}

	log.Printf("init %s: ensuring git clone at %s", name, pkg.LocalClone)
	if err := gitops.EnsureClone(pkg.GitURL, pkg.LocalClone); err != nil {
		log.Printf("init %s: git clone/fetch failed: %v", name, err)
		return
	}

	version, err := s.detectVersion(pkg)
	if err != nil {
		log.Printf("init %s: version detection failed: %v", name, err)
		return
	}

	state := s.states[name]
	if changed := state.setVersion(version); changed {
		log.Printf("init %s: version set to %s", name, version)
		s.persistState()
	} else {
		log.Printf("init %s: version confirmed %s", name, version)
	}
}
