package server

import "time"

// webhookAllowed returns true and records the current time if the per-package
// cooldown has elapsed since the last accepted webhook. Returns false otherwise.
func (s *Server) webhookAllowed(name string) bool {
	now := time.Now()
	cooldown := s.cfg.Server.WebhookCooldown

	for {
		last, loaded := s.webhookLast.Load(name)
		if loaded && now.Sub(last.(time.Time)) < cooldown {
			return false
		}
		// Try to swap in the new timestamp (compare-and-swap loop).
		if !loaded {
			if _, exists := s.webhookLast.LoadOrStore(name, now); !exists {
				return true
			}
			// Another goroutine stored first — re-check.
			continue
		}
		if s.webhookLast.CompareAndSwap(name, last, now) {
			return true
		}
		// Lost the race — re-check.
	}
}
