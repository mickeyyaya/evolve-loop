package bridge

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// bridgeManifestDir is the writable manifest-override directory consulted
// before the embedded set: EVOLVE_BRIDGE_MANIFEST_DIR, else
// .evolve/bridge-manifests. `bridge add-rule` writes here; LoadManifest
// reads here first so operator-added rules take effect.
func bridgeManifestDir() string {
	if d := os.Getenv("EVOLVE_BRIDGE_MANIFEST_DIR"); d != "" {
		return d
	}
	return filepath.Join(".evolve", "bridge-manifests")
}

// manifests/*.json are the per-CLI capability manifests (ported verbatim
// from tools/agent-bridge/lib/manifests/). Embedding them makes the Go
// bridge self-contained — no dependency on the bash tree after the M7
// cutover deletes it.
//
//go:embed manifests/*.json
var embeddedManifests embed.FS

// manifestSource is the (test-swappable) source of manifest files. The
// embed.FS satisfies it in production; tests inject a fake to drive the
// ReadFile/ReadDir error branches that the always-valid embed can't.
type manifestSource interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

var manifestFS manifestSource = embeddedManifests

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
	// Operator override (from `bridge add-rule`) wins over the embedded set.
	if data, err := os.ReadFile(filepath.Join(bridgeManifestDir(), cli+".json")); err == nil {
		return parseManifest(cli, data)
	}
	data, err := manifestFS.ReadFile("manifests/" + cli + ".json")
	if err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: no manifest for cli=%s", cli)
	}
	return parseManifest(cli, data)
}

// parseManifest unmarshals + validates manifest bytes. Split out so the
// JSON-error and missing-field branches are testable (the embedded
// manifests are all valid, so they'd otherwise be unreachable).
func parseManifest(cli string, data []byte) (Manifest, error) {
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
