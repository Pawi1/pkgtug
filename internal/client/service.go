package client

import (
	"fmt"
	"os/exec"
)

type initSystem int

const (
	initUnknown initSystem = iota
	initSystemd
	initOpenRC
)

func detectInit() initSystem {
	if path, err := exec.LookPath("systemctl"); err == nil && path != "" {
		if err := exec.Command("systemctl", "--version").Run(); err == nil {
			return initSystemd
		}
	}
	if path, err := exec.LookPath("rc-service"); err == nil && path != "" {
		_ = path
		return initOpenRC
	}
	return initUnknown
}

func stopService(name string) error {
	switch detectInit() {
	case initSystemd:
		return runCmd("systemctl", "stop", name)
	case initOpenRC:
		return runCmd("rc-service", name, "stop")
	default:
		return fmt.Errorf("no supported init system found; stop %q manually", name)
	}
}

func startService(name string) error {
	switch detectInit() {
	case initSystemd:
		return runCmd("systemctl", "start", name)
	case initOpenRC:
		return runCmd("rc-service", name, "start")
	default:
		return fmt.Errorf("no supported init system found; start %q manually", name)
	}
}

func runCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w\n%s", name, args, err, out)
	}
	return nil
}
