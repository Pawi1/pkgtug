package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Manifest struct {
	Version  string                       `json:"version"`
	Binaries map[string]map[string]Binary `json:"binaries"` // component → platform → binary
}

type Binary struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

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
