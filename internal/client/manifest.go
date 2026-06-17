package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pawi1/pkgtug/internal/manifest"
)

// Type aliases so callers share a single definition from internal/manifest.
type Manifest = manifest.Manifest
type Binary = manifest.Binary

var httpClient = &http.Client{Timeout: 30 * time.Second}

func FetchManifest(serverURL, packageName string) (*Manifest, error) {
	url := serverURL + "/tug/repo/" + packageName + "/manifest.json"
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch manifest: status %d", resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &m, nil
}
