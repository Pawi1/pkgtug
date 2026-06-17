package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/server"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "/etc/pkgtug/server.yaml", "path to server config file")
	flag.Parse()

	cfg, err := config.LoadServer(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv := server.New(cfg)
	log.Printf("pkgtug-server %s listening on %s", version, cfg.Server.Listen)
	if err := http.ListenAndServe(cfg.Server.Listen, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}
