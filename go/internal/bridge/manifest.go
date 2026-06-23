package bridge

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/paths"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

var bridgeManifestDirFn = func() string {
	layout := paths.ResolveFromEnv()
	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err == nil {
		if dir := pol.BridgeConfig().ManifestDir; dir != "" {
			return dir
		}
	}
	return filepath.Join(layout.EvolveDir, "bridge-manifests")
}

// bridgeManifestDir is the writable manifest-override directory consulted
// before the embedded set. `bridge add-rule` writes here; LoadManifest reads
// here first so operator-added rules take effect.
func bridgeManifestDir() string {
	return bridgeManifestDirFn()
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
	// Once marks a fire-once prompt (e.g. a boot-time trust dialog): after it has
	// auto-responded a single time it is NOT re-evaluated, because its dismissed
	// text lingers in the captured scrollback (bootScrollback) and would otherwise
	// re-match every poll and trip the loop guard. Recurring prompts (per-edit
	// approval, AskUserQuestion menus) leave this false.
	Once bool `json:"once"`
}

// Manifest is a per-CLI capability manifest (schema v1). Drives probe
// tiering, the REPL prompt marker, and the auto-respond rule set.
type Manifest struct {
	CLI    string `json:"cli"`
	Binary string `json:"binary"`
	// Transport classifies the execution model: "tmux" for interactive REPL
	// drivers that require a tmux session, "headless" for non-interactive
	// subprocess drivers. Use Manifest.IsTmux() rather than inspecting the
	// CLI name string directly — that is the closed abstraction point for
	// this distinction across the entire codebase.
	Transport          string              `json:"transport,omitempty"`
	BinaryMinVersion   string              `json:"binary_min_version"`
	DefaultTier        string              `json:"default_tier"`
	TierDependencies   map[string][]string `json:"tier_dependencies"`
	PromptMarker       string              `json:"prompt_marker"`
	DefaultModel       string              `json:"default_model"`
	DefaultArgs        []string            `json:"default_args"`
	InteractivePrompts []ManifestPrompt    `json:"interactive_prompts"`
	Stub               bool                `json:"stub"`
	// ModelTierMap translates the abstract, provider-neutral model tier
	// (fast|balanced|deep — the same vocabulary profiles' model_tier_default
	// + model_tier_envelope already use) to this CLI's concrete model
	// identifier. Each CLI's table is the single source of truth for that
	// translation; the realizer at realizer.go:realizeScalar is generic.
	// Consumed by ParamSpec.From == "model_tier_map" (canonical) — the
	// legacy spelling "tier_alias" is accepted for one release for
	// backward compat with operator-installed v1 override manifests.
	// See docs/architecture/adr/0022-launch-intent-realizer.md.
	ModelTierMap map[string]string `json:"model_tier_map,omitempty"`
	// ChatGPTSafeModels lists the concrete model IDs a ChatGPT/subscription
	// account can reliably use for this CLI. When the resolved auth mode is
	// "chatgpt" and the realized -m model is NOT in this set, the driver clamps
	// it to ChatGPTDefaultModel. Empty → no clamp (API-key-only CLIs, or no
	// constraint). codex's model picker/docs advertise models (gpt-5.4, gpt-5.5,
	// gpt-5.3-codex) that the live backend 400-rejects on ChatGPT accounts by
	// plan tier (multiple open OpenAI issues); this set is the proven-safe
	// subset. See docs/incidents/cycle-142-* and the codex-chatgpt-model-support
	// research dossier.
	ChatGPTSafeModels []string `json:"chatgpt_safe_models,omitempty"`
	// ChatGPTDefaultModel is the model substituted when a ChatGPT-auth launch
	// resolves to a non-safe model. MUST be a member of ChatGPTSafeModels.
	ChatGPTDefaultModel string `json:"chatgpt_default_model,omitempty"`
	// Params is the declarative per-CLI realization table: how each high-level
	// LaunchIntent parameter maps to this CLI's launch flags / REPL input /
	// controller hints. Absent param → no-op. See ADR-0022 + realizer.go.
	Params map[string]ParamSpec `json:"params,omitempty"`
	// Controls is the per-CLI control mapping table: how each ABSTRACT control
	// event (usage|status|clean_ctx|…) maps to THIS CLI's concrete slash
	// command. It is the data half of the CLI-control abstraction — the
	// pipeline names an abstract event, this table resolves the command, and
	// the CLI implementation stays hidden behind the abstraction (Adapter).
	// Absent event → Control() reports not-found, which the Controller turns
	// into a clean ErrUnsupported (e.g. ollama has no usage command).
	Controls map[string]ControlSpec `json:"controls,omitempty"`
}

// ControlSpec is one abstract-event → concrete-command mapping (a manifest
// `controls.<event>` entry). Send is the literal command pasted into the REPL;
// Await names the pane condition to wait for after sending (default
// "prompt_marker"); ExhaustedRegex, when set, is the conservative classifier a
// consumer matches against the captured response to decide the family is
// quota-capped (empty → the consumer treats the response as informational only).
type ControlSpec struct {
	Send           string `json:"send"`
	Await          string `json:"await,omitempty"`
	ExhaustedRegex string `json:"exhausted_regex,omitempty"`
}

// Control resolves the ControlSpec for an abstract event. ok=false when the CLI
// declares no mapping for event (or no controls block at all) — a nil map reads
// cleanly, so callers need no nil guard.
func (m Manifest) Control(event string) (ControlSpec, bool) {
	spec, ok := m.Controls[event]
	return spec, ok
}

// LoadManifest reads and validates the embedded manifest for cli, then overlays
// any LIVE model-catalog tier models over its ModelTierMap (see
// catalog_overlay.go). The overlay is a no-op when no catalog exists, so this
// is byte-identical to the raw load until `evolve models refresh` writes one.
// Error messages mirror lib/manifest-loader.sh.
func LoadManifest(cli string) (Manifest, error) {
	m, err := loadManifestRaw(cli)
	if err != nil {
		return m, err
	}
	return overlayManifestCatalog(m), nil
}

// loadManifestRaw is the unmodified loader: operator override > embedded set.
func loadManifestRaw(cli string) (Manifest, error) {
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
// manifests are all valid, so they'd otherwise be unreachable). Defers
// the actual work to parseManifestWithStderr with os.Stderr.
func parseManifest(cli string, data []byte) (Manifest, error) {
	return parseManifestWithStderr(cli, data, os.Stderr)
}

// parseManifestWithStderr is the testable seam for parseManifest. The
// stderr writer captures the v1 deprecation warning so test suites can
// assert it without polluting os.Stderr. Production calls go through
// parseManifest with os.Stderr.
func parseManifestWithStderr(cli string, data []byte, stderr io.Writer) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: invalid JSON for cli=%s: %w", cli, err)
	}
	if m.CLI == "" || m.Binary == "" {
		return Manifest{}, fmt.Errorf("bridge:manifest: missing required fields (cli, binary) for %s", cli)
	}
	// v1 → v2 schema compat (cycle-124 followup): a manifest declaring the
	// legacy `tier_aliases` key — with the Anthropic-leaked vocabulary
	// `{haiku|sonnet|opus → native}` — is read into a sidecar struct,
	// translated to the canonical `fast|balanced|deep` keys, and merged
	// into ModelTierMap. We only translate when ModelTierMap is empty
	// (v1-only); a manifest declaring both keys keeps ModelTierMap as the
	// source of truth without warnings. One deprecation line per manifest.
	if len(m.ModelTierMap) == 0 {
		var v1 struct {
			TierAliases map[string]string `json:"tier_aliases"`
		}
		// Second Unmarshal of the same bytes: an error here is impossible
		// given the first Unmarshal into `m` already validated the JSON
		// shape; a struct-tag mismatch just leaves v1.TierAliases at nil.
		// Explicit discard documents the intent for future readers (per
		// cycle-124 PR 2 review).
		_ = json.Unmarshal(data, &v1)
		if len(v1.TierAliases) > 0 {
			m.ModelTierMap = translateV1TierAliases(v1.TierAliases)
			fmt.Fprintf(stderr, "[bridge:manifest] DEPRECATED v1 schema for cli=%s: `tier_aliases` is deprecated; migrate to `model_tier_map` with fast/balanced/deep keys. See ADR-0022.\n", cli)
		}
	}
	return m, nil
}

// translateV1TierAliases maps the legacy Anthropic-named tier keys to the
// canonical abstract vocabulary. Non-standard keys (e.g. operator-custom
// "large") pass through verbatim. Delegates per-key translation to
// translateV1TierKey so the 3-entry mapping is the single source of truth
// (also called from realizer.go's intent-vocabulary fallback ladder —
// keeping it canonical here avoids silent drift if a fourth legacy alias
// is ever added).
func translateV1TierAliases(v1 map[string]string) map[string]string {
	out := make(map[string]string, len(v1))
	for k, v := range v1 {
		out[translateV1TierKey(k)] = v
	}
	return out
}

// translateV1TierKey is the canonical 3-entry haiku/sonnet/opus →
// fast/balanced/deep mapping. Pass-through for anything else (so custom
// operator tiers survive the migration unchanged). Pure function exported
// at package scope so both the parse-time shim (translateV1TierAliases)
// and the realize-time fallback ladder (realizer.legacyTierAlias)
// reference the same table.
func translateV1TierKey(k string) string {
	switch k {
	case "haiku":
		return "fast"
	case "sonnet":
		return "balanced"
	case "opus":
		return "deep"
	default:
		return k
	}
}

// IsTmux reports whether this manifest represents a tmux-driven REPL driver.
// Prefer this over inspecting CLI name strings (e.g. strings.HasSuffix(cli, "-tmux"))
// so the transport classification has a single authoritative source.
func (m Manifest) IsTmux() bool {
	return m.Transport == "tmux"
}

// IsTmuxDriver reports whether cli is a tmux-driven REPL driver by consulting
// its manifest's Transport field. Falls back to the "-tmux" suffix check when
// the manifest cannot be loaded (e.g. an unknown operator-installed CLI).
func IsTmuxDriver(cli string) bool {
	if m, err := LoadManifest(cli); err == nil {
		return m.IsTmux()
	}
	return strings.HasSuffix(cli, "-tmux")
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
