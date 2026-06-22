package server

import (
	"net/http"
	"strings"
)

// corsMiddleware adds CORS headers to public (non-worker) endpoints.
// Worker endpoints (which mutate state) are intentionally excluded.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	origins := s.cfg.Server.CORSOrigins
	if len(origins) == 0 {
		return next // CORS disabled
	}

	allowAll := len(origins) == 1 && origins[0] == "*"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply CORS to public read endpoints.
		if !isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				for _, o := range origins {
					if o == origin {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Add("Vary", "Origin")
						break
					}
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPublicEndpoint(path string) bool {
	return strings.HasPrefix(path, "/tug/repo/") ||
		path == "/tug/packages" ||
		path == "/healthz"
}
