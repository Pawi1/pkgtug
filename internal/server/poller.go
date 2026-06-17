package server

import (
	"context"
	"log"
	"time"

	"github.com/pawi1/pkgtug/internal/gitops"
)

// StartPolling launches a background goroutine for each package that has
// poll_interval configured. It returns immediately; goroutines stop when ctx
// is cancelled.
func (s *Server) StartPolling(ctx context.Context) {
	for name, pkg := range s.packages {
		if pkg.PollInterval <= 0 {
			continue
		}
		log.Printf("poller %s: starting (interval=%s)", name, pkg.PollInterval)
		go s.pollLoop(ctx, name, pkg.PollInterval)
	}
}

func (s *Server) pollLoop(ctx context.Context, name string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollPackage(ctx, name)
		}
	}
}

func (s *Server) pollPackage(_ context.Context, name string) {
	pkg := s.packages[name]

	mu := s.packageMu(name)
	if !mu.TryLock() {
		log.Printf("poller %s: fetch already in progress, skipping", name)
		return
	}
	defer mu.Unlock()

	if err := gitops.Fetch(pkg.LocalClone, pkg.VersionSource.Type, pkg.VersionSource.Name); err != nil {
		log.Printf("poller %s: git fetch failed: %v", name, err)
		return
	}

	version, err := s.detectVersion(pkg)
	if err != nil {
		log.Printf("poller %s: version detection failed: %v", name, err)
		return
	}

	if changed := s.states[name].setVersion(version); changed {
		log.Printf("poller %s: new version detected: %s", name, version)
		s.persistState()
	}
}
