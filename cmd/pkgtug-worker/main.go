package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pawi1/pkgtug/internal/worker"
)

var version = "dev"

func main() {
	serverURL := flag.String("server", "", "pkgtug-server base URL (required)")
	secret := flag.String("secret", "", "worker shared secret (required)")
	platform := flag.String("platform", "", "platform string, e.g. linux-x64 (auto-detected if empty)")
	workDir := flag.String("work-dir", "/var/cache/pkgtug-worker", "directory for git clones and builds")
	interval := flag.Duration("interval", 30*time.Second, "poll interval")
	flag.Parse()

	if *serverURL == "" || *secret == "" {
		log.Fatal("--server and --secret are required")
	}

	plat := *platform
	if plat == "" {
		var err error
		plat, err = worker.PlatformFromUname()
		if err != nil {
			log.Fatalf("auto-detect platform: %v", err)
		}
		log.Printf("detected platform: %s", plat)
	}

	if err := os.MkdirAll(*workDir, 0o755); err != nil {
		log.Fatalf("work-dir: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("pkgtug-worker %s", version)
	worker.Run(ctx, worker.Config{
		ServerURL: *serverURL,
		Secret:    *secret,
		Platform:  plat,
		WorkDir:   *workDir,
		Interval:  *interval,
	})
}
