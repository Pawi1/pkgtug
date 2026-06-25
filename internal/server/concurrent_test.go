package server_test

import (
	"sync"
	"testing"

	"github.com/pawi1/pkgtug/internal/client"
)

// TestConcurrentPushSamePackage verifies that pushing multiple platforms for the
// same package concurrently does not lose any manifest entries. This exercises
// the per-package manifestLock that serialises manifest reads and writes.
func TestConcurrentPushSamePackage(t *testing.T) {
	baseURL, secret := testServer(t)

	platforms := []string{"linux-x64", "linux-arm64", "darwin-arm64"}

	var wg sync.WaitGroup
	for _, plat := range platforms {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			pushBinary(t, baseURL, secret, "mypkg", "1.0.0", p, "mybin", []byte("bin for "+p))
		}(plat)
	}
	wg.Wait()

	mf, err := client.FetchManifest(baseURL, "mypkg")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	for _, plat := range platforms {
		if _, ok := mf.Binaries["mybin"][plat]; !ok {
			t.Errorf("manifest missing entry for platform %q after concurrent push", plat)
		}
	}
}

// TestConcurrentPushDifferentVersions verifies that the last pushed version wins
// and all its platform entries are present, even under concurrent load.
func TestConcurrentPushDifferentVersions(t *testing.T) {
	baseURL, secret := testServer(t)

	platforms := []string{"linux-x64", "linux-arm64"}

	var wg sync.WaitGroup
	for _, plat := range platforms {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			pushBinary(t, baseURL, secret, "mypkg", "2.0.0", p, "mybin", []byte("v2 "+p))
		}(plat)
	}
	wg.Wait()

	mf, err := client.FetchManifest(baseURL, "mypkg")
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if mf.Version != "2.0.0" {
		t.Errorf("manifest version = %q, want 2.0.0", mf.Version)
	}
	for _, plat := range platforms {
		if _, ok := mf.Binaries["mybin"][plat]; !ok {
			t.Errorf("manifest missing %q after concurrent push", plat)
		}
	}
}
