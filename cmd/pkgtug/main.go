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

	app := newApp(cfg, *configPath, *statePath, state, platform)

	switch cmd {
	case "remote":
		err = app.cmdRemote(args)
	case "search":
		err = app.cmdSearch(args)
	case "install":
		err = app.cmdInstall(args)
	case "check":
		err = app.cmdCheck(args)
	case "update":
		err = app.cmdUpdate(args)
	case "status":
		err = app.cmdStatus(args)
	case "rollback":
		err = app.cmdRollback(args)
	case "uninstall":
		err = app.cmdUninstall(args)
	case "pin":
		err = app.cmdPin(args)
	case "autoupdate":
		err = app.cmdAutoupdate(args)
	case "daemon":
		err = app.cmdDaemon(args)
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
  remote add <name> <url>  add a package server
  remote remove <name>     remove a package server
  remote list              list configured servers

  search [<query>]         search available packages across all remotes
  install [<remote>:]<package>[/<component>]  install a package
  check <package/component>   check for an update
  update <package/component>  update to latest (--all for all packages)
  autoupdate [<package/component>]    mark/unmark package for daemon autoupdate (--remove to unmark)
  daemon                   run as daemon — auto-updates marked packages (--interval, default 15m)
  status                   show installed packages and their remotes
  pin <package/component>       lock version, skip in update --all (--unpin to release)
  uninstall <package/component>  remove a package from state (--remove-binary to also delete file)
  rollback <package/component>  restore previous binary from backup
  version                  print pkgtug version

Flags:
`, version)
	flag.PrintDefaults()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pkgtug: "+format, args...)
	os.Exit(1)
}
