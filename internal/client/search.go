package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type PackageEntry struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func FetchPackages(serverURL string) ([]PackageEntry, error) {
	resp, err := httpClient.Get(serverURL + "/tug/packages")
	if err != nil {
		return nil, fmt.Errorf("fetch packages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch packages: status %d", resp.StatusCode)
	}
	var list []PackageEntry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode packages: %w", err)
	}
	return list, nil
}

// FilterPackages returns entries whose name contains query (case-insensitive).
func FilterPackages(list []PackageEntry, query string) []PackageEntry {
	if query == "" {
		return list
	}
	q := strings.ToLower(query)
	var out []PackageEntry
	for _, e := range list {
		if strings.Contains(strings.ToLower(e.Name), q) {
			out = append(out, e)
		}
	}
	return out
}
