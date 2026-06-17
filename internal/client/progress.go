package client

import (
	"io"
	"log"
)

// Progress is implemented by both the plain-text logger (cron/non-TTY)
// and the TUI renderer (interactive terminal).
type Progress interface {
	Log(format string, args ...any)
	StartSpinner(msg string)
	StopSpinner()
	DownloadWriter(name string, size int64) io.Writer // wraps response body; nil = direct copy
	FinishDownload()
}

// PlainProgress writes to the standard logger — safe for cron/non-TTY use.
type PlainProgress struct{}

func (PlainProgress) Log(format string, args ...any)        { log.Printf(format, args...) }
func (PlainProgress) StartSpinner(msg string)               { log.Printf("%s...", msg) }
func (PlainProgress) StopSpinner()                          {}
func (PlainProgress) DownloadWriter(_ string, _ int64) io.Writer { return nil }
func (PlainProgress) FinishDownload()                       {}
