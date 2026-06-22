package server

import (
	"testing"
)

func TestValidPathComponent(t *testing.T) {
	valid := []string{
		"myapp", "v1.2.3", "linux-amd64", "server",
		"26.07.01-stable", "abc123", "my-package_v2",
	}
	for _, s := range valid {
		if err := validPathComponent(s); err != nil {
			t.Errorf("validPathComponent(%q) unexpected error: %v", s, err)
		}
	}

	invalid := []struct {
		input string
		desc  string
	}{
		{"", "empty"},
		{".", "dot"},
		{"..", "double-dot"},
		{"a/b", "forward slash"},
		{"a\\b", "backslash"},
		{"a\x00b", "null byte"},
		{"/etc/passwd", "absolute path"},
		{"../secret", "traversal"},
	}
	for _, tc := range invalid {
		if err := validPathComponent(tc.input); err == nil {
			t.Errorf("validPathComponent(%q) expected error (%s), got nil", tc.input, tc.desc)
		}
	}
}

func TestUnderRoot(t *testing.T) {
	root := "/data/packages/myapp"

	ok := []string{
		"/data/packages/myapp/v1/linux-amd64/server",
		"/data/packages/myapp/v1/linux-amd64",
		"/data/packages/myapp/v1",
	}
	for _, p := range ok {
		if err := underRoot(root, p); err != nil {
			t.Errorf("underRoot(%q) unexpected error: %v", p, err)
		}
	}

	bad := []string{
		"/data/packages/myapp-other/v1/linux-amd64/server",
		"/data/packages",
		"/data/packages/myapp/../other/evil",
		"/etc/passwd",
		"/data/packages/myapp2",
	}
	for _, p := range bad {
		if err := underRoot(root, p); err == nil {
			t.Errorf("underRoot(%q) expected error, got nil", p)
		}
	}
}
