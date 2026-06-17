package server

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validPathComponent rejects path segments that could cause traversal:
// empty strings, segments containing "/" or "\", and the special "." / ".." values.
func validPathComponent(s string) error {
	if s == "" {
		return fmt.Errorf("path component must not be empty")
	}
	if strings.ContainsAny(s, "/\\") {
		return fmt.Errorf("path component must not contain slashes: %q", s)
	}
	if s == "." || s == ".." {
		return fmt.Errorf("path component must not be . or ..: %q", s)
	}
	if strings.Contains(s, "\x00") {
		return fmt.Errorf("path component must not contain null bytes")
	}
	return nil
}

// underRoot verifies that resolved is inside root after cleaning both paths.
// Prevents any remaining edge cases after per-component validation.
func underRoot(root, resolved string) error {
	clean := filepath.Clean(resolved)
	cleanRoot := filepath.Clean(root) + string(filepath.Separator)
	if !strings.HasPrefix(clean+string(filepath.Separator), cleanRoot) {
		return fmt.Errorf("path %q escapes root %q", resolved, root)
	}
	return nil
}
