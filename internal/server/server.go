package server

import (
	"net/http"
	"sync"

	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/notify"
)

type Server struct {
	cfg      *config.ServerConfig
	packages map[string]*config.Package
	states   map[string]*packageState
	jobs     *jobRegistry
	tg       *notify.Telegram
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
	tg := cfg.Telegram
	return &Server{
		cfg:      cfg,
		packages: pkgs,
		states:   states,
		jobs:     newJobRegistry(),
		tg:       notify.NewTelegram(tg.BotToken, tg.ChatID),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// public — no auth
	mux.HandleFunc("POST /tug/fetch/{name}", s.handleFetch)
	mux.HandleFunc("GET /tug/repo/{name}/manifest.json", s.handleManifest)
	mux.HandleFunc("GET /tug/repo/{name}/binaries/{version}/{platform}/{component}", s.handleBinaryDownload)
	mux.HandleFunc("GET /tug/packages", s.handlePackageList)
	// authenticated — requires Authorization: Bearer <worker_secret>
	mux.HandleFunc("POST /tug/repo/{name}/push", s.handlePush)
	mux.HandleFunc("GET /tug/build/next", s.handleBuildNext)
	mux.HandleFunc("POST /tug/build/{job_id}/result", s.handleBuildResult)
	return s.corsMiddleware(mux)
}

func (s *Server) packageMu(name string) *sync.Mutex {
	v, _ := s.fetchMu.LoadOrStore(name, &sync.Mutex{})
	return v.(*sync.Mutex)
}
