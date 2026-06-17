package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GHRelease represents the relevant fields of a GitHub Releases API response.
type GHRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []GHAsset `json:"assets"`
}

// GHAsset is a single downloadable file attached to a GitHub release.
type GHAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// FetchLatestGHRelease returns the latest release for "owner/repo".
// Respects the GITHUB_TOKEN environment variable for auth (higher rate limits, private repos).
func FetchLatestGHRelease(ownerRepo string) (*GHRelease, error) {
	url := "https://api.github.com/repos/" + ownerRepo + "/releases/latest"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("github: repo %q not found or no releases", ownerRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: status %d", resp.StatusCode)
	}

	var rel GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("github api: decode: %w", err)
	}
	return &rel, nil
}

// InstallableAssets filters out checksums, signatures and other non-binary files.
func InstallableAssets(assets []GHAsset) []GHAsset {
	skip := []string{".sha256", ".sha512", ".md5", ".sig", ".asc", ".pem", ".sbom", ".txt", ".json"}
	var out []GHAsset
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		bad := false
		for _, s := range skip {
			if strings.HasSuffix(lower, s) {
				bad = true
				break
			}
		}
		if !bad {
			out = append(out, a)
		}
	}
	return out
}

// MatchGHAsset returns the index of the best-matching asset for the given platform,
// or -1 if no confident match is found (caller should show a picker).
//
// Platform strings follow pkgtug convention: linux-x64, linux-arm64, darwin-x64, darwin-arm64.
func MatchGHAsset(assets []GHAsset, platform string) int {
	os_, arch := splitPlatform(platform)
	best := -1
	bestScore := 0

	for i, a := range assets {
		score := scoreAsset(strings.ToLower(a.Name), os_, arch)
		if score > bestScore {
			bestScore = score
			best = i
		}
	}

	// Only auto-select when the match is unambiguous (score ≥ 2: OS + arch both matched).
	if bestScore >= 2 {
		return best
	}
	return -1
}

func splitPlatform(platform string) (os_, arch string) {
	parts := strings.SplitN(platform, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return platform, ""
}

func scoreAsset(name, os_, arch string) int {
	score := 0

	// OS match
	switch os_ {
	case "linux":
		if strings.Contains(name, "linux") {
			score++
		} else {
			return 0
		}
	case "darwin":
		if strings.Contains(name, "darwin") || strings.Contains(name, "macos") || strings.Contains(name, "osx") {
			score++
		} else {
			return 0
		}
	case "windows":
		if strings.Contains(name, "windows") || strings.Contains(name, "win") {
			score++
		} else {
			return 0
		}
	}

	// Arch match
	switch arch {
	case "x64":
		if strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") || strings.Contains(name, "x64") {
			score++
		}
	case "arm64":
		if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
			score++
		}
	case "arm":
		if strings.Contains(name, "arm") && !strings.Contains(name, "arm64") && !strings.Contains(name, "aarch64") {
			score++
		}
	}

	return score
}

// FindChecksumAsset looks for a companion checksum file for the given asset name.
// Returns nil if not found.
func FindChecksumAsset(all []GHAsset, assetName string) *GHAsset {
	candidates := []string{
		assetName + ".sha256",
		assetName + ".sha512",
		"checksums.txt",
		"SHA256SUMS",
		"sha256sums.txt",
	}
	for _, c := range candidates {
		for i := range all {
			if strings.EqualFold(all[i].Name, c) {
				return &all[i]
			}
		}
	}
	return nil
}
