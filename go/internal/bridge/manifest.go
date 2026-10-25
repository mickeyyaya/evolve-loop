package bridge

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

func bridgeManifestDir() string {
	return bridgeManifestDirFn()
}

//go:embed manifests/*.json
var embeddedManifests embed.FS

type manifestSource interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

var manifestFS manifestSource = embeddedManifests

type ManifestPrompt struct {
	Name         string `json:"name"`
	Regex        string `json:"regex"`
	ResponseKeys string `json:"response_keys"`
	Policy       string `json:"policy"`
	Note         string `json:"note"`
	Once         bool   `json:"once"`
	TailLines    int    `json:"tail_lines"`
}

type ModelFreshness struct {
	Prefer   string   `json:"prefer,omitempty"`
	AliasIDs []string `json:"alias_ids,omitempty"`
	Note     string   `json:"note,omitempty"`
}

type Manifest struct {
	CLI                     string                   `json:"cli"`
	Binary                  string                   `json:"binary"`
	Transport               string                   `json:"transport,omitempty"`
	BinaryMinVersion        string                   `json:"binary_min_version"`
	UpdateArgv              []string                 `json:"update_argv,omitempty"`
	AutoUpdateOffEnv        string                   `json:"auto_update_off_env,omitempty"`
	ProbeBootRetries        int                      `json:"probe_boot_retries,omitempty"`
	DefaultTier             string                   `json:"default_tier"`
	TierDependencies        map[string][]string      `json:"tier_dependencies"`
	PromptMarker            string                   `json:"prompt_marker"`
	DefaultModel            string                   `json:"default_model"`
	DefaultArgs             []string                 `json:"default_args"`
	DefaultEnv              map[string]string        `json:"default_env,omitempty"`
	EnvPassthrough          []string                 `json:"env_passthrough,omitempty"`
	MCPConfigFiles          []string                 `json:"mcp_config_files,omitempty"`
	InteractivePrompts      []ManifestPrompt         `json:"interactive_prompts"`
	TransientRegex          string                   `json:"transient_regex,omitempty"`
	BusyLineRegex           string                   `json:"busy_line_regex,omitempty"`
	TokenLineRegex          string                   `json:"token_line_regex,omitempty"`
	ModelLabelRegex         string                   `json:"model_label_regex,omitempty"`
	Stub                    bool                     `json:"stub"`
	Toolless                bool                     `json:"toolless,omitempty"`
	ModelTierMap            map[string]string        `json:"model_tier_map,omitempty"`
	ModelTierMapFrom        string                   `json:"model_tier_map_from,omitempty"`
	ChatGPTSafeModels       []string                 `json:"chatgpt_safe_models,omitempty"`
	ChatGPTDefaultModel     string                   `json:"chatgpt_default_model,omitempty"`
	ModelFreshness          ModelFreshness           `json:"model_freshness,omitempty"`
	ModelFamily             string                   `json:"model_family,omitempty"`
	Params                  map[string]ParamSpec     `json:"params,omitempty"`
	Controls                map[string]ControlSpec   `json:"controls,omitempty"`
	LaunchModelVerification *LaunchModelVerification `json:"launch_model_verification,omitempty"`
}

type ControlSpec struct {
	Send            string                 `json:"send"`
	Await           string                 `json:"await,omitempty"`
	ExhaustedRegex  string                 `json:"exhausted_regex,omitempty"`
	DriftProbeRegex string                 `json:"drift_probe_regex,omitempty"`
	Windows         *quotastate.WindowSpec `json:"windows,omitempty"`
}

func (m Manifest) Control(event string) (ControlSpec, bool) {
	spec, ok := m.Controls[event]
	return spec, ok
}

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

func parseManifest(cli string, data []byte) (Manifest, error) {
	return parseManifestWithStderr(cli, data, os.Stderr)
}

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
	if err := validateEnvPassthrough(cli, m.EnvPassthrough); err != nil {
		return Manifest{}, err
	}
	if err := validateMCPConfigFiles(cli, m.MCPConfigFiles); err != nil {
		return Manifest{}, err
	}
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
		_ = json.Unmarshal(data, &v1)
		if len(v1.TierAliases) > 0 {
			m.ModelTierMap = translateV1TierAliases(v1.TierAliases)
			fmt.Fprintf(stderr, "[bridge:manifest] DEPRECATED v1 schema for cli=%s: `tier_aliases` is deprecated; migrate to `model_tier_map` with fast/balanced/deep keys. See ADR-0022.\n", cli)
		}
	}
	return m, nil
}

func validateMCPConfigFiles(cli string, entries []string) error {
	for _, entry := range entries {
		if strings.HasPrefix(entry, "/") && !strings.HasPrefix(entry, "~/") || slices.Contains(strings.Split(entry, "/"), "..") {
			return fmt.Errorf("bridge:manifest: mcp_config_files entry %q for cli=%s must be relative or start with ~/ and carry no .. segment", entry, cli)
		}
	}
	return nil
}

func validateEnvPassthrough(cli string, names []string) error {
	for _, name := range names {
		if !isShellIdentifier(name) || isReservedEnvKey(name) {
			return fmt.Errorf("bridge:manifest: env_passthrough name %q for cli=%s is not a shell identifier, or it is a loop, bridge or credential variable", name, cli)
		}
	}
	return nil
}

var credentialEnvKeys = map[string]bool{"ANTHROPIC_API_KEY": true, "ANTHROPIC_BASE_URL": true, "OPENAI_API_KEY": true}

func isReservedEnvKey(key string) bool {
	return credentialEnvKeys[key] || strings.HasPrefix(key, "EVOLVE_") || strings.HasPrefix(key, "BRIDGE_")
}

func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

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

func translateV1TierAliases(v1 map[string]string) map[string]string {
	out := make(map[string]string, len(v1))
	for k, v := range v1 {
		out[translateV1TierKey(k)] = v
	}
	return out
}

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

func (m Manifest) IsTmux() bool {
	return m.Transport == "tmux"
}

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
