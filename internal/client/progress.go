package client

import (
	"io"
	"log"
)

// ConflictAction is the user's decision when a config/service file conflict is detected.
type ConflictAction int

const (
	ConflictUseNew      ConflictAction = iota // overwrite with new version from server
	ConflictKeepCurrent                       // keep user's version; save new as .pkgtug-new
	ConflictEdit                              // open $EDITOR on current file (.pkgtug-new for reference)
	ConflictAbort                             // abort update for this package
)

// Progress is implemented by both the plain-text logger (cron/non-TTY)
// and the TUI renderer (interactive terminal).
type Progress interface {
	Log(format string, args ...any)
	StartSpinner(msg string)
	StopSpinner()
	DownloadWriter(name string, size int64) io.Writer // wraps response body; nil = direct copy
	FinishDownload()
	// ResolveConflict is called when a file on disk was user-modified and a new version
	// is also available. diffText is the output of diff -u (may be empty if diff unavailable).
	// Non-interactive implementations should return ConflictKeepCurrent.
	ResolveConflict(key, path, diffText string) ConflictAction
}

// PlainProgress writes to the standard logger — safe for cron/non-TTY use.
type PlainProgress struct{}

func (PlainProgress) Log(format string, args ...any)             { log.Printf(format, args...) }
func (PlainProgress) StartSpinner(msg string)                    { log.Printf("%s...", msg) }
func (PlainProgress) StopSpinner()                               {}
func (PlainProgress) DownloadWriter(_ string, _ int64) io.Writer { return nil }
func (PlainProgress) FinishDownload()                            {}
func (PlainProgress) ResolveConflict(key, path, _ string) ConflictAction {
	log.Printf("conflict: %s: %s was modified locally; new version saved as %s.pkgtug-new", key, path, path)
	return ConflictKeepCurrent
}
