package looppreflight

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releaseLookupTimeout = 3 * time.Second

var npmLatestReleaseURLs = map[string]string{
	"claude": "https://registry.npmjs.org/@anthropic-ai/claude-code/latest",
	"codex":  "https://registry.npmjs.org/@openai/codex/latest",
}

func DefaultLatestRelease(bin string) (string, error) {
	return latestReleaseFrom(&http.Client{Timeout: releaseLookupTimeout}, npmLatestReleaseURLs, bin)
}

func latestReleaseFrom(client *http.Client, urls map[string]string, bin string) (string, error) {
	url, ok := urls[bin]
	if !ok {
		return "", fmt.Errorf("no release source configured for %s", bin)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Version == "" {
		return "", errors.New("empty version in release payload")
	}
	return payload.Version, nil
}

func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	tokens := strings.Split(v, ".")
	parts := make([]int, 0, len(tokens))
	for _, tok := range tokens {
		var n int
		for i := 0; i < len(tok); i++ {
			if tok[i] >= '0' && tok[i] <= '9' {
				n = n*10 + int(tok[i]-'0')
			} else {
				break
			}
		}
		parts = append(parts, n)
	}
	return parts
}

func IsLagging(installed, latest string) bool {
	if latest == "" || latest == "unavailable" {
		return false
	}
	inst := parseVersionParts(installed)
	lat := parseVersionParts(latest)
	maxLen := max(len(inst), len(lat))
	for i := 0; i < maxLen; i++ {
		var pInst, pLat int
		if i < len(inst) {
			pInst = inst[i]
		}
		if i < len(lat) {
			pLat = lat[i]
		}
		if pLat > pInst {
			return true
		}
		if pLat < pInst {
			return false
		}
	}
	return false
}

func IsLaggingBy(installed, latest string, patchThreshold int) bool {
	if !IsLagging(installed, latest) {
		return false
	}
	inst := padVersion(parseVersionParts(installed))
	lat := padVersion(parseVersionParts(latest))
	if inst[0] != lat[0] || inst[1] != lat[1] {
		return true
	}
	return lat[2]-inst[2] > patchThreshold
}

func padVersion(parts []int) [3]int {
	var out [3]int
	copy(out[:], parts)
	return out
}
