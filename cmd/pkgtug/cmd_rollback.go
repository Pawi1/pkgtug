package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pawi1/pkgtug/internal/client"
)

func (a *App) cmdRollback(args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: pkgtug rollback <package/component>")
	}
	key := fs.Arg(0)

	entry := a.state[key]
	if entry == nil {
		return fmt.Errorf("%s: not installed", key)
	}
	if entry.BackupDir == "" {
		return fmt.Errorf("%s: no backup directory configured", key)
	}

	_, component, err := splitComponentKey(key)
	if err != nil {
		return err
	}

	backupPath := filepath.Join(entry.BackupDir, component+".bak")
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup not found at %s: %w", backupPath, err)
	}

	p := a.newProgress()

	if entry.ServiceName != "" {
		p.Log("stopping service %s", entry.ServiceName)
		if err := client.StopService(entry.ServiceName); err != nil {
			return fmt.Errorf("stop service: %w", err)
		}
	}

	p.Log("restoring %s → %s", backupPath, entry.BinaryPath)
	if err := copyFile(backupPath, entry.BinaryPath); err != nil {
		return fmt.Errorf("restore binary: %w", err)
	}

	if entry.ServiceName != "" {
		p.Log("starting service %s", entry.ServiceName)
		if err := client.StartService(entry.ServiceName); err != nil {
			return fmt.Errorf("start service: %w", err)
		}
	}

	fmt.Printf("✓ %s rolled back\n", key)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func splitComponentKey(key string) (pkg, component string, err error) {
	return client.SplitKey(key)
}
