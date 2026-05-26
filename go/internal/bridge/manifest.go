package bridge

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// manifests/*.json are the per-CLI capability manifests (ported verbatim
// from tools/agent-bridge/lib/manifests/). Embedding them makes the Go
// bridge self-contained — no dependency on the bash tree after the M7
// cutover deletes it.
//
//go:embed manifests/*.json
var manifestFS embed.FS

// ManifestPrompt is one interactive_prompts[] rule consumed by the
// auto-respond engine. ResponseKeys "" (JSON null) means escalate.
type ManifestPrompt struct {
	Name         string `json:"name"`
	Regex        string `json:"regex"`
	ResponseKeys string `json:"response_keys"`
	Policy       string `json:"policy"` // auto_respond | escalate
	Note         string `json:"note"`
}

// Manifest is a per-CLI capability manifest (schema v1). Drives probe
// tiering, the REPL prompt marker, and the auto-respond rule set.
type Manifest struct {
	CLI                string              `json:"cli"`
	Binary             string              `json:"binary"`
	BinaryMinVersion   string              `json:"binary_min_version"`
	DefaultTier        string              `json:"default_tier"`
	TierDependencies   map[string][]string `json:"tier_dependencies"`
	PromptMarker       string              `json:"prompt_marker"`
	DefaultModel       string              `json:"default_model"`
	DefaultArgs        []string            `json:"default_args"`
	InteractivePrompts []ManifestPrompt    `json:"interactive_prompts"`
	Stub               bool                `json:"stub"`
}

// LoadManifest reads and validates the embedded manifest for cli. Error
// messages mirror lib/manifest-loader.sh.
func LoadManifest(cli string) (Manifest, error) {
	if cli == "" {
		return Manifest{}, fmt.Errorf("bridge:manifest: empty cli name")
	}
	data, err := manifestFS.ReadFile("manifests/" + cli + ".json")
	if err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: no manifest for cli=%s", cli)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: invalid JSON for cli=%s: %w", cli, err)
	}
	if m.CLI == "" || m.Binary == "" {
		return Manifest{}, fmt.Errorf("bridge:manifest: missing required fields (cli, binary) for %s", cli)
	}
	return m, nil
}

// ManifestNames returns the sorted set of CLI names with an embedded
// manifest (one per manifests/*.json).
func ManifestNames() []string {
	entries, err := manifestFS.ReadDir("manifests")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	sort.Strings(out)
	return out
}
