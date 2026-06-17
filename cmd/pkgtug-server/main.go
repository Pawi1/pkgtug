package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg)

	log.Printf("pkgtug-server %s: initialising packages", version)
	srv.Init(ctx)
	srv.StartPolling(ctx)

	log.Printf("pkgtug-server %s listening on %s", version, cfg.Server.Listen)
	httpSrv := &http.Server{Addr: cfg.Server.Listen, Handler: srv.Handler()}

	go func() {
		<-ctx.Done()
		log.Println("pkgtug-server: shutting down")
		httpSrv.Shutdown(context.Background())
	}()

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(1)
	}
}
