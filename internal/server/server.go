package server

import (
	"net/http"
	"sync"

	"github.com/pawi1/pkgtug/internal/config"
)

type Server struct {
	cfg      *config.ServerConfig
	packages map[string]*config.Package
	fetchMu  sync.Map // per-package mutex to serialize concurrent fetches
}

func New(cfg *config.ServerConfig) *Server {
	pkgs := make(map[string]*config.Package, len(cfg.Packages))
	for i := range cfg.Packages {
		pkgs[cfg.Packages[i].Name] = &cfg.Packages[i]
	}
	return &Server{cfg: cfg, packages: pkgs}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tug/fetch/{name}", s.handleFetch)
	return mux
}

func (s *Server) packageMu(name string) *sync.Mutex {
	v, _ := s.fetchMu.LoadOrStore(name, &sync.Mutex{})
	return v.(*sync.Mutex)
}
