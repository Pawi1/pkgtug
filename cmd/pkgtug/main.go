package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/worker"
)

var version = "dev"

const (
	defaultConfigPath = "/etc/pkgtug/config.yaml"
	defaultStatePath  = "/var/lib/pkgtug/state.json"
)

func main() {
	configPath := flag.String("config", defaultConfigPath, "client config file")
	statePath := flag.String("state", defaultStatePath, "state file")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	// Commands that don't need config/state
	switch cmd {
	case "version":
		fmt.Println("pkgtug", version)
		return
	}

	cfg, err := client.LoadConfig(*configPath)
	if err != nil {
		fatalf("config: %v\n", err)
	}

	state, err := client.LoadState(*statePath)
	if err != nil {
		fatalf("state: %v\n", err)
	}

	platform, err := worker.PlatformFromUname()
	if err != nil {
		fatalf("detect platform: %v\n", err)
	}

	app := &App{
		cfg:        cfg,
		statePath:  *statePath,
		state:      state,
		platform:   platform,
	}

	switch cmd {
	case "check":
		err = app.cmdCheck(args)
	case "update":
		err = app.cmdUpdate(args)
	case "status":
		err = app.cmdStatus(args)
	case "rollback":
		err = app.cmdRollback(args)
	case "search":
		err = app.cmdSearch(args)
	case "install":
		err = app.cmdInstall(args)
	default:
		fatalf("unknown command %q — run pkgtug --help\n", cmd)
	}

	if err != nil {
		fatalf("%v\n", err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `pkgtug %s — generic binary auto-updater

Usage:
  pkgtug [flags] <command> [args]

Commands:
  search <query>           list available packages
  install <pkg/component>  install a package component
  check <pkg/component>    check for updates
  update <pkg/component>   update to latest version (use --all for all)
  status                   show installed packages and versions
  rollback <pkg/component> restore previous binary from backup
  version                  print pkgtug version

Flags:
`, version)
	flag.PrintDefaults()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pkgtug: "+format, args...)
	os.Exit(1)
}
