package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ---- runDiff ----------------------------------------------------------------

func TestRunDiff_filesDiffer(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	os.WriteFile(a, []byte("hello\n"), 0o644)
	os.WriteFile(b, []byte("world\n"), 0o644)

	out := runDiff(a, b)
	if out == "" {
		t.Error("expected non-empty diff output for differing files")
	}
}

func TestRunDiff_filesIdentical(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	os.WriteFile(a, []byte("same\n"), 0o644)
	os.WriteFile(b, []byte("same\n"), 0o644)

	if out := runDiff(a, b); out != "" {
		t.Errorf("expected empty output for identical files, got %q", out)
	}
}

// TestRunDiff_missingFile verifies that when diff exits with code 2 (tool error),
// the error message is NOT returned as diff text — the caller must not see it.
func TestRunDiff_missingFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	os.WriteFile(a, []byte("hello\n"), 0o644)

	out := runDiff(a, filepath.Join(dir, "nonexistent"))
	if out != "" {
		t.Errorf("expected empty string when diff fails (exit 2), got %q", out)
	}
}

// ---- verifyGHChecksum -------------------------------------------------------

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func checksumServer(t *testing.T, body string) (url string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, body)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "bin-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(content)
	f.Close()
	return f.Name()
}

func TestVerifyGHChecksum_matchingLine(t *testing.T) {
	content := []byte("binary content v1")
	tmpFile := writeTempFile(t, content)
	want := sha256hex(content)

	url := checksumServer(t, fmt.Sprintf("%s  myapp-linux-amd64.tar.gz\n", want))
	if err := verifyGHChecksum(tmpFile, "myapp-linux-amd64.tar.gz", url); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestVerifyGHChecksum_binaryMode verifies the "*filename" (binary mode) format
// that some projects use in their checksum files.
func TestVerifyGHChecksum_binaryMode(t *testing.T) {
	content := []byte("binary content v2")
	tmpFile := writeTempFile(t, content)
	want := sha256hex(content)

	url := checksumServer(t, fmt.Sprintf("%s *myapp-linux-amd64.tar.gz\n", want))
	if err := verifyGHChecksum(tmpFile, "myapp-linux-amd64.tar.gz", url); err != nil {
		t.Errorf("binary-mode format should be accepted: %v", err)
	}
}

// TestVerifyGHChecksum_noMatch verifies that when the checksum file contains no
// entry for the requested asset, an error is returned rather than silent success.
func TestVerifyGHChecksum_noMatch(t *testing.T) {
	tmpFile := writeTempFile(t, []byte("some binary"))
	deadHash := "0000000000000000000000000000000000000000000000000000000000000000"

	url := checksumServer(t, fmt.Sprintf("%s  other-asset.tar.gz\n", deadHash))
	if err := verifyGHChecksum(tmpFile, "myapp-linux-amd64.tar.gz", url); err == nil {
		t.Error("expected error when asset name not found in checksum file, got nil")
	}
}

func TestVerifyGHChecksum_singleHash(t *testing.T) {
	content := []byte("single-hash release")
	tmpFile := writeTempFile(t, content)
	want := sha256hex(content)

	url := checksumServer(t, want)
	if err := verifyGHChecksum(tmpFile, "any-name.tar.gz", url); err != nil {
		t.Errorf("single 64-char hash should be accepted: %v", err)
	}
}

func TestVerifyGHChecksum_wrongHash(t *testing.T) {
	tmpFile := writeTempFile(t, []byte("actual content"))
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"

	url := checksumServer(t, fmt.Sprintf("%s  myapp.tar.gz\n", wrong))
	if err := verifyGHChecksum(tmpFile, "myapp.tar.gz", url); err == nil {
		t.Error("expected error for hash mismatch, got nil")
	}
}

// ---- atomicReplace ----------------------------------------------------------

func TestAtomicReplace_basic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	os.WriteFile(src, []byte("new content"), 0o755)
	os.WriteFile(dst, []byte("old content"), 0o755)

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "new content" {
		t.Errorf("dst content = %q, want %q", string(got), "new content")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("src should not exist after rename-based replace")
	}
}

func TestAtomicReplace_createsParentDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "sub", "nested", "bin")

	os.WriteFile(src, []byte("content"), 0o755)

	if err := atomicReplace(src, dst); err != nil {
		t.Fatalf("atomicReplace with missing parent dirs: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst not created: %v", err)
	}
}

// ---- backupBinary -----------------------------------------------------------

func TestBackupBinary_createsBackup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "mybin")
	backupDir := filepath.Join(dir, "backups")

	os.WriteFile(src, []byte("original binary"), 0o755)

	path, err := backupBinary(src, backupDir, "mybin")
	if err != nil {
		t.Fatalf("backupBinary: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "original binary" {
		t.Errorf("backup content = %q, want %q", string(got), "original binary")
	}
}
