package server

import (
	"net/http"
	"time"
)

// limitUpload wraps r.Body with MaxBytesReader when max_upload_size is configured.
// If the limit is 0 (unlimited), the body is left unchanged.
func (s *Server) limitUpload(w http.ResponseWriter, r *http.Request) {
	if limit := int64(s.cfg.Server.MaxUploadSize); limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
}

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
