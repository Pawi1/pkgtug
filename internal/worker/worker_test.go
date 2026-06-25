package worker

import (
	"path/filepath"
	"strings"
	"testing"
)

// ---- scrubEnv ---------------------------------------------------------------

func TestScrubEnv_removesSecrets(t *testing.T) {
	env := []string{
		"HOME=/root",
		"PKGTUG_SECRET=supersecret",
		"PATH=/usr/bin:/bin",
		"PKGTUG_SERVER=http://internal:8080",
		"USER=builder",
	}
	got := scrubEnv(env, "PKGTUG_SECRET", "PKGTUG_SERVER")

	for _, e := range got {
		if strings.HasPrefix(e, "PKGTUG_SECRET=") {
			t.Errorf("PKGTUG_SECRET should be scrubbed, still present: %q", e)
		}
		if strings.HasPrefix(e, "PKGTUG_SERVER=") {
			t.Errorf("PKGTUG_SERVER should be scrubbed, still present: %q", e)
		}
	}
}

func TestScrubEnv_keepsOtherVars(t *testing.T) {
	env := []string{"HOME=/root", "PKGTUG_SECRET=s", "PATH=/bin"}
	got := scrubEnv(env, "PKGTUG_SECRET")

	want := map[string]bool{"HOME=/root": true, "PATH=/bin": true}
	for _, e := range got {
		delete(want, e)
	}
	if len(want) != 0 {
		t.Errorf("variables missing after scrub: %v", want)
	}
}

func TestScrubEnv_emptyKeys(t *testing.T) {
	env := []string{"FOO=bar", "BAZ=qux"}
	got := scrubEnv(env)
	if len(got) != len(env) {
		t.Errorf("scrubEnv with no keys changed env: got %v", got)
	}
}

// ---- bin.Path traversal -----------------------------------------------------

// TestBinPathTraversal exercises the path-escape check added to postSuccess.
// The check: filepath.Join(cloneDir, bin.Path) must stay under cloneDir.
func TestBinPathTraversal(t *testing.T) {
	cloneDir := filepath.Join("/work", "myapp")

	cases := []struct {
		binPath    string
		shouldPass bool
	}{
		{"dist/mybin", true},
		{"subdir/nested/binary", true},
		{"binary", true},
		{"../../etc/shadow", false},
		{"../sibling", false},
		{"../myapp-evil/bin", false},
	}

	for _, c := range cases {
		resolved := filepath.Join(cloneDir, c.binPath)
		escapes := !strings.HasPrefix(resolved, cloneDir+string(filepath.Separator))
		if escapes == c.shouldPass {
			t.Errorf("path %q: escapes=%v, want escapes=%v", c.binPath, escapes, !c.shouldPass)
		}
	}
}
