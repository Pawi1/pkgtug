package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// MetaManifest is a remotely-hosted list of pkgtug remotes.
// Host it on GitHub Pages, a gist, or any static file server.
type MetaManifest struct {
	Remotes []MetaRemote `json:"remotes"`
}

type MetaRemote struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Priority int    `json:"priority,omitempty"` // lower = higher priority; 0 = last
}

// FetchMeta downloads and parses a meta-manifest from url.
func FetchMeta(url string) (*MetaManifest, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch meta %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch meta %s: status %d", url, resp.StatusCode)
	}
	var m MetaManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode meta %s: %w", url, err)
	}
	for i := range m.Remotes {
		m.Remotes[i].URL = strings.TrimRight(m.Remotes[i].URL, "/")
	}
	return &m, nil
}

// EffectiveRemotes returns the merged remote list for use in resolve/search.
// Order: locally-configured remotes first (always win on name conflict), then
// remotes from each meta_url in config order, sorted by priority within each
// meta source (lower priority number = tried first; 0 = unset = last).
// Failed meta fetches are silently skipped so local remotes still work offline.
func EffectiveRemotes(cfg *Config) []Remote {
	seen := make(map[string]bool, len(cfg.Remotes))
	out := make([]Remote, 0, len(cfg.Remotes))

	for _, r := range cfg.Remotes {
		seen[r.Name] = true
		out = append(out, r)
	}

	for _, metaURL := range cfg.MetaURLs {
		m, err := FetchMeta(metaURL)
		if err != nil {
			continue
		}
		sorted := make([]MetaRemote, len(m.Remotes))
		copy(sorted, m.Remotes)
		sort.SliceStable(sorted, func(i, j int) bool {
			pi, pj := sorted[i].Priority, sorted[j].Priority
			if pi == 0 {
				pi = 1 << 30
			}
			if pj == 0 {
				pj = 1 << 30
			}
			return pi < pj
		})
		for _, mr := range sorted {
			if seen[mr.Name] || mr.URL == "" {
				continue
			}
			seen[mr.Name] = true
			out = append(out, Remote{Name: mr.Name, URL: mr.URL})
		}
	}
	return out
}
