package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pawi1/pkgtug/internal/client"
	"github.com/pawi1/pkgtug/internal/config"
	"github.com/pawi1/pkgtug/internal/server"
)

// testServer starts an httptest server backed by a direct_push package "mypkg"
// with a single component "mybin". The BaseURL in the config is set to the
// test server URL so manifest binary URLs point back to the same server.
func testServer(t *testing.T) (baseURL, secret string) {
	t.Helper()
	dataDir := t.TempDir()
	secret = "test-secret"

	var handler http.Handler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)

	cfg := &config.ServerConfig{
		Server: config.ServerSection{
			Listen:       ":0",
			BaseURL:      ts.URL,
			DataDir:      dataDir,
			WorkerSecret: secret,
		},
		Packages: []config.Package{
			{
				Name:       "mypkg",
				DirectPush: true,
				Binaries:   []config.Binary{{Component: "mybin", Path: "dist/mybin"}},
			},
		},
	}
	srv := server.New(cfg)
	handler = srv.Handler()
	return ts.URL, secret
}

func pushBinary(t *testing.T, baseURL, secret, pkg, version, platform, component string, content []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("version", version)
	mw.WriteField("platform", platform)
	mw.WriteField("component", component)
	fw, err := mw.CreateFormFile("file", component)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fw.Write(content)
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/tug/repo/"+pkg+"/push", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("push: status %d: %s", resp.StatusCode, body)
	}
}

func hexSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// TestPushManifestVersion verifies that after a push the manifest reflects the
// correct version and SHA256 of the uploaded binary.
func TestPushManifestVersion(t *testing.T) {
	baseURL, secret := testServer(t)
	content := []byte("#!/bin/sh\necho hello v1")
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", content)

	mf, err := client.FetchManifest(baseURL, "mypkg")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if mf.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", mf.Version)
	}
	bin, ok := mf.Binaries["mybin"]["linux-x64"]
	if !ok {
		t.Fatal("manifest missing mybin/linux-x64")
	}
	if want := hexSHA256(content); bin.SHA256 != want {
		t.Errorf("SHA256 = %q, want %q", bin.SHA256, want)
	}
	if bin.URL == "" {
		t.Error("manifest URL is empty")
	}
}

// TestDownloadMatchesManifestSHA downloads the binary from the URL in the
// manifest and verifies the content and SHA256 match what was pushed.
func TestDownloadMatchesManifestSHA(t *testing.T) {
	baseURL, secret := testServer(t)
	content := []byte("#!/bin/sh\necho mybin v1")
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", content)

	mf, _ := client.FetchManifest(baseURL, "mypkg")
	bin := mf.Binaries["mybin"]["linux-x64"]

	resp, err := http.Get(bin.URL)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, content) {
		t.Error("downloaded content does not match pushed content")
	}
	if gotSHA := hexSHA256(got); gotSHA != bin.SHA256 {
		t.Errorf("downloaded SHA256 = %q, want %q", gotSHA, bin.SHA256)
	}
}

// TestPushUpdatesManifestVersion verifies that pushing a new version updates
// the manifest version and SHA256.
func TestPushUpdatesManifestVersion(t *testing.T) {
	baseURL, secret := testServer(t)
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", []byte("v1"))

	v2 := []byte("v2 binary content")
	pushBinary(t, baseURL, secret, "mypkg", "2.0.0", "linux-x64", "mybin", v2)

	mf, err := client.FetchManifest(baseURL, "mypkg")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if mf.Version != "2.0.0" {
		t.Errorf("version = %q, want 2.0.0", mf.Version)
	}
	bin := mf.Binaries["mybin"]["linux-x64"]
	if want := hexSHA256(v2); bin.SHA256 != want {
		t.Errorf("SHA256 after update = %q, want %q", bin.SHA256, want)
	}
}

// TestPackageList verifies that the package list endpoint lists the package
// after at least one binary has been pushed.
func TestPackageList(t *testing.T) {
	baseURL, secret := testServer(t)
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", []byte("bin"))

	resp, err := http.Get(baseURL + "/tug/packages")
	if err != nil {
		t.Fatalf("package list: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("package list status %d", resp.StatusCode)
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode package list: %v", err)
	}
	for _, e := range entries {
		if e.Name == "mypkg" {
			return
		}
	}
	t.Error("package list does not contain mypkg")
}

// TestLatestRedirect verifies that /binaries/latest/… redirects to the
// versioned download URL.
func TestLatestRedirect(t *testing.T) {
	baseURL, secret := testServer(t)
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", []byte("binary"))

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirect.Get(baseURL + "/tug/repo/mypkg/binaries/latest/linux-x64/mybin")
	if err != nil {
		t.Fatalf("latest request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Error("no Location header in redirect response")
	}
}

// TestClientCheckDetectsUpdate verifies that client.Check reports an update
// available when the installed version differs from the server version.
func TestClientCheckDetectsUpdate(t *testing.T) {
	baseURL, secret := testServer(t)
	pushBinary(t, baseURL, secret, "mypkg", "2.0.0", "linux-x64", "mybin", []byte("v2"))

	state := client.State{
		"mypkg/mybin": &client.InstallEntry{
			Remote:           baseURL,
			InstalledVersion: "1.0.0",
			BinaryPath:       filepath.Join(t.TempDir(), "mybin"),
		},
	}

	result, err := client.Check(baseURL, state, "mypkg/mybin", "linux-x64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.UpdateAvailable {
		t.Error("UpdateAvailable = false, want true")
	}
	if result.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want 2.0.0", result.LatestVersion)
	}
}

// TestClientUpdateInstallsBinary verifies the full update flow: the binary is
// downloaded, written to BinaryPath, and the state entry version is updated.
func TestClientUpdateInstallsBinary(t *testing.T) {
	baseURL, secret := testServer(t)
	content := []byte("#!/bin/sh\necho hello from mybin")
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", content)

	installPath := filepath.Join(t.TempDir(), "mybin")
	state := client.State{
		"mypkg/mybin": &client.InstallEntry{
			Remote:     baseURL,
			BinaryPath: installPath,
		},
	}

	updated, err := client.Update(baseURL, "", state, "mypkg/mybin", "linux-x64", client.PlainProgress{})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated {
		t.Error("Update returned false, expected binary to be installed")
	}

	got, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Error("installed binary content does not match pushed content")
	}
	if state["mypkg/mybin"].InstalledVersion != "1.0.0" {
		t.Errorf("state version = %q, want 1.0.0", state["mypkg/mybin"].InstalledVersion)
	}
}

// TestClientUpdateNoopWhenCurrent verifies that Update returns (false, nil)
// when the installed version already matches the server version.
func TestClientUpdateNoopWhenCurrent(t *testing.T) {
	baseURL, secret := testServer(t)
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", []byte("v1"))

	state := client.State{
		"mypkg/mybin": &client.InstallEntry{
			Remote:           baseURL,
			InstalledVersion: "1.0.0",
			BinaryPath:       filepath.Join(t.TempDir(), "mybin"),
		},
	}

	updated, err := client.Update(baseURL, "", state, "mypkg/mybin", "linux-x64", client.PlainProgress{})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated {
		t.Error("Update returned true, expected no-op when already at latest")
	}
}

// TestClientUpdateSHA256Verified verifies that SHA256 of the installed binary
// matches the manifest after a successful update.
func TestClientUpdateSHA256Verified(t *testing.T) {
	baseURL, secret := testServer(t)
	content := []byte("#!/bin/sh\necho sha test")
	pushBinary(t, baseURL, secret, "mypkg", "1.0.0", "linux-x64", "mybin", content)

	installPath := filepath.Join(t.TempDir(), "mybin")
	state := client.State{
		"mypkg/mybin": &client.InstallEntry{
			Remote:     baseURL,
			BinaryPath: installPath,
		},
	}

	if _, err := client.Update(baseURL, "", state, "mypkg/mybin", "linux-x64", client.PlainProgress{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	mf, _ := client.FetchManifest(baseURL, "mypkg")
	wantSHA := mf.Binaries["mybin"]["linux-x64"].SHA256

	gotSHA, err := client.SHA256File(installPath)
	if err != nil {
		t.Fatalf("SHA256File: %v", err)
	}
	if gotSHA != wantSHA {
		t.Errorf("installed SHA256 = %q, want %q", gotSHA, wantSHA)
	}
}
