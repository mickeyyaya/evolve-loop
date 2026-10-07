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

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
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

// bridgeManifestDir is the writable manifest-override directory consulted before the embedded set.
func bridgeManifestDir() string {
	return bridgeManifestDirFn()
}

//go:embed manifests/*.json
var embeddedManifests embed.FS

// manifestSource lets tests inject a fake to drive ReadFile/ReadDir error branches the always-valid embed can't.
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
	Policy       string `json:"policy"`
	Note         string `json:"note"`
	Once         bool   `json:"once"`
	// TailLines restricts matching to the pane's last n lines (0 = whole capture): a live modal sits at the
	// bottom, and a fixed-size byte window can't express that scrolled-off text is unmatchable regardless of length.
	TailLines int `json:"tail_lines"`
}

// Manifest is a per-CLI capability manifest (schema v1). Drives probe
// ModelFreshness declares how a CLI's "latest within a lineage" is chosen: "alias" resolves the family alias
// at launch (fresher than any cached catalog id); the zero value keeps the newest concrete version.
type ModelFreshness struct {
	// Prefer selects the freshness rule: "alias" or "" (newest_version).
	Prefer string `json:"prefer,omitempty"`
	// AliasIDs are the ids the CLI self-resolves, in preference order.
	AliasIDs []string `json:"alias_ids,omitempty"`
	// Note documents the evidence for the declaration (informational).
	Note string `json:"note,omitempty"`
}

type Manifest struct {
	CLI    string `json:"cli"`
	Binary string `json:"binary"`
	// Transport is "tmux" for interactive REPL drivers or "headless" for non-interactive subprocess drivers;
	// use Manifest.IsTmux() rather than the CLI name string.
	Transport        string              `json:"transport,omitempty"`
	BinaryMinVersion string              `json:"binary_min_version"`
	UpdateArgv       []string            `json:"update_argv,omitempty"`
	AutoUpdateOffEnv string              `json:"auto_update_off_env,omitempty"`
	ProbeBootRetries int                 `json:"probe_boot_retries,omitempty"`
	DefaultTier      string              `json:"default_tier"`
	TierDependencies map[string][]string `json:"tier_dependencies"`
	PromptMarker     string              `json:"prompt_marker"`
	DefaultModel     string              `json:"default_model"`
	DefaultArgs      []string            `json:"default_args"`
	// DefaultEnv is the always-on environment of the CLI process, as DefaultArgs is its always-on flags.
	DefaultEnv         map[string]string `json:"default_env,omitempty"`
	InteractivePrompts []ManifestPrompt  `json:"interactive_prompts"`
	// TransientRegex recognizes a temporary upstream failure on the phase pane, distinct from the permanent
	// quota wall; empty disables it.
	TransientRegex  string `json:"transient_regex,omitempty"`
	BusyLineRegex   string `json:"busy_line_regex,omitempty"`
	TokenLineRegex  string `json:"token_line_regex,omitempty"`
	ModelLabelRegex string `json:"model_label_regex,omitempty"`
	Stub            bool   `json:"stub"`
	Toolless        bool   `json:"toolless,omitempty"`
	// ModelTierMap translates the abstract fast|balanced|deep model tier to this CLI's concrete model id;
	// each CLI's table is the single source of truth for that translation.
	ModelTierMap map[string]string `json:"model_tier_map,omitempty"`
	// ModelTierMapFrom names another manifest whose model_tier_map this one adopts when it declares none of
	// its own; see resolveTierMapFrom.
	ModelTierMapFrom string `json:"model_tier_map_from,omitempty"`
	// ChatGPTSafeModels lists the model ids a ChatGPT/subscription account can reliably use; empty means no clamp.
	ChatGPTSafeModels []string `json:"chatgpt_safe_models,omitempty"`
	// ChatGPTDefaultModel replaces a non-safe model on ChatGPT auth; it must itself be a member of ChatGPTSafeModels.
	ChatGPTDefaultModel string `json:"chatgpt_default_model,omitempty"`
	// ModelFreshness is a fact about the CLI binary, not an operator preference; cmd/evolve's composition root
	// maps it to modelquery.FreshnessPolicy, and modelquery never imports bridge.
	ModelFreshness ModelFreshness `json:"model_freshness,omitempty"`
	ModelFamily    string         `json:"model_family,omitempty"`
	// Params is the declarative per-CLI realization table: how each LaunchIntent parameter maps to this CLI's
	// flags, REPL input or controller hints; an absent param is a no-op.
	// See ADR-0022.
	Params map[string]ParamSpec `json:"params,omitempty"`
	// Controls is the per-CLI control mapping table: an abstract event (usage|status|clean_ctx|…) to this
	// CLI's concrete slash command; an absent event reports not-found.
	Controls                map[string]ControlSpec   `json:"controls,omitempty"`
	LaunchModelVerification *LaunchModelVerification `json:"launch_model_verification,omitempty"`
}

// ControlSpec is one abstract-event → concrete-command manifest entry: Send is pasted into the REPL, Await
// names the pane condition to wait for (default "prompt_marker"), and ExhaustedRegex, when set, classifies
// the response as a quota wall.
type ControlSpec struct {
	Send           string `json:"send"`
	Await          string `json:"await,omitempty"`
	ExhaustedRegex string `json:"exhausted_regex,omitempty"`
	// DriftProbeRegex is a broad, deliberately-loose quota-wall heuristic used only by the drift alarm
	// (exhaustion_drift.go); empty disables the alarm for this CLI.
	DriftProbeRegex string                 `json:"drift_probe_regex,omitempty"`
	Windows         *quotastate.WindowSpec `json:"windows,omitempty"`
}

// Control resolves the ControlSpec for an abstract event; ok=false when the CLI declares no mapping (or no
// controls block at all) — a nil map reads cleanly.
func (m Manifest) Control(event string) (ControlSpec, bool) {
	spec, ok := m.Controls[event]
	return spec, ok
}

// LoadManifest reads and validates the embedded manifest for cli, then overlays any live model-catalog tier
// models over its ModelTierMap; the overlay is a no-op until `evolve models refresh` writes a catalog.
func LoadManifest(cli string) (Manifest, error) {
	m, err := loadManifestRaw(cli)
	if err != nil {
		return m, err
	}
	m, err = resolveTierMapFrom(cli, m)
	if err != nil {
		return Manifest{}, err
	}
	return overlayManifestCatalog(m), nil
}

// resolveTierMapFrom adopts m.ModelTierMapFrom's model_tier_map when m declares none of its own (a copy,
// never a shared reference); an unresolvable pointer is a manifest error, never a silent empty map.
func resolveTierMapFrom(cli string, m Manifest) (Manifest, error) {
	if m.ModelTierMapFrom == "" || len(m.ModelTierMap) != 0 {
		return m, nil
	}
	from, err := loadManifestRaw(m.ModelTierMapFrom)
	if err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: cli=%s model_tier_map_from=%q: %w", cli, m.ModelTierMapFrom, err)
	}
	if len(from.ModelTierMap) == 0 {
		return Manifest{}, fmt.Errorf("bridge:manifest: cli=%s model_tier_map_from=%q declares no model_tier_map", cli, m.ModelTierMapFrom)
	}
	m.ModelTierMap = make(map[string]string, len(from.ModelTierMap))
	for k, v := range from.ModelTierMap {
		m.ModelTierMap[k] = v
	}
	return m, nil
}

// loadManifestRaw is the unmodified loader: operator override > embedded set.
func loadManifestRaw(cli string) (Manifest, error) {
	if cli == "" {
		return Manifest{}, fmt.Errorf("bridge:manifest: empty cli name")
	}
	data, err := resolvedManifestBytes(cli)
	if err != nil {
		return Manifest{}, err
	}
	return parseManifest(cli, data)
}

// parseManifest unmarshals and validates manifest bytes; split out so the JSON-error and missing-field
// branches are testable, since the embedded manifests are all valid otherwise.
func parseManifest(cli string, data []byte) (Manifest, error) {
	return parseManifestWithStderr(cli, data, os.Stderr)
}

// parseManifestWithStderr is parseManifest's testable seam: the stderr writer captures the v1 deprecation
// warning so tests can assert it without polluting os.Stderr.
func parseManifestWithStderr(cli string, data []byte, stderr io.Writer) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("bridge:manifest: invalid JSON for cli=%s: %w", cli, err)
	}
	if m.CLI == "" || m.Binary == "" {
		return Manifest{}, fmt.Errorf("bridge:manifest: missing required fields (cli, binary) for %s", cli)
	}
	if m.ProbeBootRetries < 0 {
		return Manifest{}, fmt.Errorf("bridge:manifest: probe_boot_retries for cli=%s is %d; it counts retries, so it cannot be negative", cli, m.ProbeBootRetries)
	}
	for key, value := range m.DefaultEnv {
		if !isShellIdentifier(key) {
			return Manifest{}, fmt.Errorf("bridge:manifest: default_env key %q for cli=%s is not a shell identifier", key, cli)
		}
		if isReservedEnvKey(key) {
			return Manifest{}, fmt.Errorf("bridge:manifest: default_env key %q for cli=%s is reserved: the loop's, the bridge's and the credential variables are not a manifest's to set", key, cli)
		}
		if hasControlByte(value) {
			return Manifest{}, fmt.Errorf("bridge:manifest: default_env value for %q (cli=%s) carries a control byte; it is typed into a pane", key, cli)
		}
	}
	// A manifest declaring the legacy `tier_aliases` key translates it to `model_tier_map` only when
	// ModelTierMap is empty; a manifest declaring both keeps ModelTierMap as the source of truth.
	if err := validatePaneVocabulary(cli, m); err != nil {
		return Manifest{}, err
	}
	if err := validateLaunchModelVerification(cli, m); err != nil {
		return Manifest{}, err
	}
	if len(m.ModelTierMap) == 0 {
		var v1 struct {
			TierAliases map[string]string `json:"tier_aliases"`
		}
		// The error is always nil: the first Unmarshal into m already validated the JSON shape, so a
		// struct-tag mismatch here can only leave v1.TierAliases at nil.
		_ = json.Unmarshal(data, &v1)
		if len(v1.TierAliases) > 0 {
			m.ModelTierMap = translateV1TierAliases(v1.TierAliases)
			fmt.Fprintf(stderr, "[bridge:manifest] DEPRECATED v1 schema for cli=%s: `tier_aliases` is deprecated; migrate to `model_tier_map` with fast/balanced/deep keys. See ADR-0022.\n", cli)
		}
	}
	return m, nil
}

// credentialEnvKeys are the variables the credential-isolation guards read before a launch; a manifest
// must not be able to set what the guards never see.
var credentialEnvKeys = map[string]bool{"ANTHROPIC_API_KEY": true, "ANTHROPIC_BASE_URL": true, "OPENAI_API_KEY": true}

// isReservedEnvKey reports whether key belongs to the loop (EVOLVE_), the bridge (BRIDGE_) or a credential.
func isReservedEnvKey(key string) bool {
	return credentialEnvKeys[key] || strings.HasPrefix(key, "EVOLVE_") || strings.HasPrefix(key, "BRIDGE_")
}

// hasControlByte reports whether s carries a C0 control byte or DEL.
func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// isShellIdentifier reports whether key can stand left of `=` in an `export` line.
func isShellIdentifier(key string) bool {
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return key != ""
}

// translateV1TierAliases maps legacy Anthropic-named tier keys to the canonical vocabulary; non-standard
// keys pass through verbatim. translateV1TierKey is the single source of truth for the mapping, also used
// by realizer.go's fallback ladder, so a new legacy alias can't drift between the two call sites.
func translateV1TierAliases(v1 map[string]string) map[string]string {
	out := make(map[string]string, len(v1))
	for k, v := range v1 {
		out[translateV1TierKey(k)] = v
	}
	return out
}

// translateV1TierKey is the canonical haiku/sonnet/opus → fast/balanced/deep mapping plus the "high"→"deep"
// alias; everything else, including "top", passes through unchanged. Both the parse-time shim and the
// realize-time fallback ladder reference this table.
func translateV1TierKey(k string) string {
	switch k {
	case "haiku":
		return "fast"
	case "sonnet":
		return "balanced"
	case "opus":
		return "deep"
	case "high":
		return "deep"
	default:
		return k
	}
}

// IsTmux reports whether this manifest represents a tmux-driven REPL driver; prefer it over inspecting the
// CLI name string, so the transport classification has a single authoritative source.
func (m Manifest) IsTmux() bool {
	return m.Transport == "tmux"
}

// IsTmuxDriver reports whether cli is a tmux-driven REPL driver by consulting its manifest's Transport
// field, falling back to the "-tmux" suffix check when the manifest cannot be loaded.
func IsTmuxDriver(cli string) bool {
	if m, err := LoadManifest(cli); err == nil {
		return m.IsTmux()
	}
	return strings.HasSuffix(cli, "-tmux")
}

func driverBinary(cli string) string {
	if m, err := loadManifestRaw(cli); err == nil {
		return m.Binary
	}
	return strings.TrimSuffix(cli, "-tmux")
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
