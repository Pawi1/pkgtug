package main

import (
	"flag"
	"fmt"
	"strings"
)

func (a *App) cmdStatus(_ []string) error {
	flag.CommandLine.Parse(nil)

	if len(a.state) == 0 {
		fmt.Println("no packages installed")
		return nil
	}

	fmt.Printf("%-35s  %-20s  %s\n", "PACKAGE/COMPONENT", "VERSION", "BINARY")
	fmt.Println(strings.Repeat("-", 80))
	for key, e := range a.state {
		fmt.Printf("%-35s  %-20s  %s\n", key, e.InstalledVersion, e.BinaryPath)
	}
	return nil
}
