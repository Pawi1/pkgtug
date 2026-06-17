package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) authWorker(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.Server.WorkerSecret)) == 1
}
