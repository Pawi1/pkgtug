package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pawi1/pkgtug/internal/client"
)

func (u *UI) ResolveConflict(key, path, diffText string) client.ConflictAction {
	u.StopSpinner()

	fmt.Fprintf(os.Stderr, "\n⚠  conflict: %s\n", key)
	fmt.Fprintf(os.Stderr, "   %s was modified locally and a new version is available.\n\n", path)

	if diffText != "" {
		fmt.Fprintln(os.Stderr, diffText)
	}

	fmt.Fprintln(os.Stderr, "  (u) use new      — overwrite with new version")
	fmt.Fprintln(os.Stderr, "  (k) keep current — save new as .pkgtug-new")
	fmt.Fprintln(os.Stderr, "  (e) edit         — open $EDITOR, new saved as .pkgtug-new")
	fmt.Fprintln(os.Stderr, "  (a) abort        — skip update for this package")

	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "  choice [k]: ")
		line, _ := r.ReadString('\n')
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "u":
			return client.ConflictUseNew
		case "k", "":
			return client.ConflictKeepCurrent
		case "e":
			return client.ConflictEdit
		case "a":
			return client.ConflictAbort
		default:
			fmt.Fprintln(os.Stderr, "  enter u, k, e or a")
		}
	}
}
