package server

import (
	"log"
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

	// Restore persisted state (best-effort — don't fail startup on error).
	if saved, err := loadPersistedState(cfg.Server.DataDir); err != nil {
		log.Printf("server: could not load persisted state: %v", err)
	} else {
		for name, ps := range saved {
			st, ok := states[name]
			if !ok {
				// Package was removed from config — skip its saved state.
				log.Printf("server: skipping persisted state for unknown package %q", name)
				continue
			}
			st.currentVersion = ps.CurrentVersion
			for plat, ver := range ps.BuiltVersions {
				st.builtVersions[plat] = ver
			}
			if ps.CurrentVersion != "" {
				log.Printf("server: restored %s — version %s, built on %d platform(s)",
					name, ps.CurrentVersion, len(ps.BuiltVersions))
			}
		}
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

func (s *Server) persistState() {
	if err := savePersistedState(s.cfg.Server.DataDir, s.states); err != nil {
		log.Printf("server: persist state: %v", err)
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
