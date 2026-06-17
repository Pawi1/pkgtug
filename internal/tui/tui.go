package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
)

// IsTerminal reports whether stdout is an interactive terminal.
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// UI provides spinner + progress bar output for interactive terminals.
// It satisfies client.Progress.
type UI struct {
	sp *spinner.Spinner
}

func New() *UI {
	sp := spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(os.Stderr))
	return &UI{sp: sp}
}

func (u *UI) Log(format string, args ...any) {
	// Pause spinner, print, resume.
	running := u.sp.Active()
	if running {
		u.sp.Stop()
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	if running {
		u.sp.Start()
	}
}

func (u *UI) StartSpinner(msg string) {
	u.sp.Suffix = "  " + msg
	u.sp.Start()
}

func (u *UI) StopSpinner() {
	u.sp.Stop()
}

func (u *UI) DownloadWriter(name string, size int64) io.Writer {
	u.sp.Stop()
	bar := progressbar.NewOptions64(size,
		progressbar.OptionSetDescription(name),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
	return bar
}

func (u *UI) FinishDownload() {
	fmt.Fprintln(os.Stderr)
}
