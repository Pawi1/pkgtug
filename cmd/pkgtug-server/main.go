package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/server"
)

var version = "dev"

func writeExampleConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(exampleConfig), 0o640)
}

const exampleConfig = `server:
  listen: ":8080"
  base_url: "https://tug.example.com"
  data_dir: "./data"
  worker_secret: "change-me"
  webhook_cooldown: 10s
  # max_upload_size: "100MB"

# telegram:
#   bot_token: ""
#   chat_id: ""

packages:
  - name: myapp
    git_url: "https://github.com/example/myapp.git"
    local_clone: "./clones/myapp"
    version_source:
      type: tag
      pattern: "v*"
    build_command: "go build -o myapp ./cmd/myapp"
    binaries:
      - component: myapp
        path: myapp
    poll_interval: 5m
    # compress: xz
`

func main() {
	cfgPath := flag.String("config", "/etc/pkgtug/server.yaml", "path to server config file")
	flag.Parse()

	if _, err := os.Stat(*cfgPath); os.IsNotExist(err) {
		if werr := writeExampleConfig(*cfgPath); werr != nil {
			log.Fatalf("config not found and could not generate example: %v", werr)
		}
		fmt.Printf("generated example config → %s\n", *cfgPath)
		fmt.Printf("edit it, then run: pkgtug-server -config %s\n", *cfgPath)
		os.Exit(0)
	}

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
	httpSrv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("pkgtug-server: shutting down")
		httpSrv.Shutdown(context.Background())
	}()

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		os.Exit(1)
	}
}
