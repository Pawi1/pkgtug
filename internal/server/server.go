package server

import (
	"net/http"
	"sync"

	"github.com/pawi1/pkgtug/internal/config"
)

type Server struct {
	cfg      *config.ServerConfig
	packages map[string]*config.Package
	states   map[string]*packageState
	jobs     *jobRegistry
	fetchMu  sync.Map // per-package mutex to serialize concurrent fetches
}

func New(cfg *config.ServerConfig) *Server {
	pkgs := make(map[string]*config.Package, len(cfg.Packages))
	states := make(map[string]*packageState, len(cfg.Packages))
	for i := range cfg.Packages {
		name := cfg.Packages[i].Name
		pkgs[name] = &cfg.Packages[i]
		states[name] = newPackageState()
	}
	return &Server{
		cfg:      cfg,
		packages: pkgs,
		states:   states,
		jobs:     newJobRegistry(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tug/fetch/{name}", s.handleFetch)
	mux.HandleFunc("GET /tug/build/next", s.handleBuildNext)
	mux.HandleFunc("POST /tug/build/{job_id}/result", s.handleBuildResult)
	return mux
}

func (s *Server) packageMu(name string) *sync.Mutex {
	v, _ := s.fetchMu.LoadOrStore(name, &sync.Mutex{})
	return v.(*sync.Mutex)
}
