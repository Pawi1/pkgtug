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

func StopService(name string) error {
	switch detectInit() {
	case initSystemd:
		return runCmd("systemctl", "stop", name)
	case initOpenRC:
		return runCmd("rc-service", name, "stop")
	default:
		return fmt.Errorf("no supported init system found; stop %q manually", name)
	}
}

func StartService(name string) error {
	switch detectInit() {
	case initSystemd:
		return runCmd("systemctl", "start", name)
	case initOpenRC:
		return runCmd("rc-service", name, "start")
	default:
		return fmt.Errorf("no supported init system found; start %q manually", name)
	}
}

// ListServices returns service names available on the current init system.
// Returns nil if the init system cannot be detected or listing fails.
func ListServices() []string {
	switch detectInit() {
	case initSystemd:
		return listSystemdServices()
	case initOpenRC:
		return listOpenRCServices()
	default:
		return nil
	}
}

func listSystemdServices() []string {
	// --no-pager --no-legend --plain for machine-readable output
	out, err := exec.Command("systemctl", "list-units",
		"--type=service", "--all", "--no-pager", "--no-legend", "--plain").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range splitLines(string(out)) {
		if line == "" {
			continue
		}
		// Fields: UNIT LOAD ACTIVE SUB DESCRIPTION
		fields := splitFields(line)
		if len(fields) < 1 {
			continue
		}
		name := fields[0]
		// Strip leading "●" marker that systemd sometimes emits
		if len(name) > 0 && name[0] > 127 {
			if len(fields) > 1 {
				name = fields[1]
			} else {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

func listOpenRCServices() []string {
	out, err := exec.Command("rc-service", "--list").Output()
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range splitLines(string(out)) {
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	inField := false
	start := 0
	for i, c := range s {
		isSpace := c == ' ' || c == '\t'
		if !inField && !isSpace {
			inField = true
			start = i
		} else if inField && isSpace {
			fields = append(fields, s[start:i])
			inField = false
		}
	}
	if inField {
		fields = append(fields, s[start:])
	}
	return fields
}

func runCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w\n%s", name, args, err, out)
	}
	return nil
}
