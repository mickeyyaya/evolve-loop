// Package profiles loads the agent profiles in .evolve/profiles/*.json, which
// pin each phase agent's CLI, model tier, tools, sandbox and budget.
package profiles

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Profile mirrors the .evolve/profiles/<name>.json schema.
type Profile struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	CLI         string   `json:"cli"`
	AllowedCLIs []string `json:"allowed_clis,omitempty"`
	// CLIFallback lists registered driver names tried in order when CLI exits
	// with a CLIFallbackOnExit code.
	CLIFallback []string `json:"cli_fallback,omitempty"`
	// CLIFallbackOnExit lists the bridge exit codes that trigger fallback; empty
	// means llmroute's default. Any other failure never reroutes. See ADR-0029.
	CLIFallbackOnExit  []int              `json:"cli_fallback_on_exit,omitempty"`
	ModelTierDefault   string             `json:"model_tier_default"`
	ModelTierEnvelope  *ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	ModelTierOverrides map[string]string  `json:"model_tier_overrides,omitempty"`
	AllowedTools       []string           `json:"allowed_tools,omitempty"`
	DisallowedTools    []string           `json:"disallowed_tools,omitempty"`
	MaxTurns           int                `json:"max_turns,omitempty"`
	ParallelEligible   bool               `json:"parallel_eligible,omitempty"`
	OutputArtifact     string             `json:"output_artifact,omitempty"`
	ResearchQuota      map[string]int     `json:"research_quota,omitempty"`
	Sandbox            *SandboxConfig     `json:"sandbox,omitempty"`
	EffortLevel        string             `json:"effort_level,omitempty"`
	// EffortOverrides maps a resolved model tier to the effort level used at
	// that tier, so a tier escalation carries its effort. See ADR-0096.
	EffortOverrides   map[string]string `json:"effort_overrides,omitempty"`
	AddDir            []string          `json:"add_dir,omitempty"`
	PermissionMode    string            `json:"permission_mode,omitempty"`
	InteractivePolicy string            `json:"interactive_policy,omitempty"`
	StreamOutput      bool              `json:"stream_output,omitempty"`
	StopCriterion     string            `json:"stop_criterion,omitempty"`
	TurnBudgetHint    int               `json:"turn_budget_hint,omitempty"`
	GeneratedFrom     string            `json:"generated_from,omitempty"`
	// SystemPrompt holds per-agent rules prepended to the prompt at launch; it
	// wins over SystemPromptFile, which resolves relative to the profile dir.
	SystemPrompt     string `json:"system_prompt,omitempty"`
	SystemPromptFile string `json:"system_prompt_file,omitempty"`
	// DigestFile names a pre-generated role-scoped digest, resolved like
	// SystemPromptFile and preferred over it when the file exists. See ADR-0023.
	DigestFile string `json:"digest_file,omitempty"`
	// Raw holds the file's original bytes, $include_policy sentinels unexpanded,
	// for keys the struct does not model; the typed tool lists are expanded.
	Raw json.RawMessage `json:"-"`
}

// ModelTierEnvelope bounds a profile's model tier escalation.
type ModelTierEnvelope struct {
	Min     string `json:"min,omitempty"`
	Default string `json:"default,omitempty"`
	Max     string `json:"max,omitempty"`
}

// SandboxConfig is the typed shape of profile.sandbox.
type SandboxConfig struct {
	Enabled          bool     `json:"enabled,omitempty"`
	ReadOnlyRepo     bool     `json:"read_only_repo,omitempty"`
	WriteSubpaths    []string `json:"write_subpaths,omitempty"`
	DenySubpaths     []string `json:"deny_subpaths,omitempty"`
	DenyReadSubpaths []string `json:"deny_read_subpaths,omitempty"`
	AllowNetwork     bool     `json:"allow_network,omitempty"`
}

// Loader resolves profile names to Profiles; a zero Loader's Get returns fs.ErrNotExist.
type Loader struct {
	fs fs.FS
}

// NewFromFS returns a Loader over fsys; a nil fsys gives the zero Loader.
func NewFromFS(fsys fs.FS) *Loader { return &Loader{fs: fsys} }

// NewFromDir returns a Loader rooted at dir; an empty dir gives the zero Loader.
func NewFromDir(dir string) *Loader {
	if dir == "" {
		return &Loader{}
	}
	return &Loader{fs: os.DirFS(dir)}
}

// Get loads <name>.json, expanding $include_policy sentinels in its tool lists.
func (l *Loader) Get(name string) (Profile, error) {
	if l.fs == nil {
		return Profile{}, fmt.Errorf("profiles: %w (no source configured)", fs.ErrNotExist)
	}
	p := name + ".json"
	raw, err := fs.ReadFile(l.fs, p)
	if err != nil {
		return Profile{}, fmt.Errorf("profiles: read %s: %w", p, err)
	}
	var prof Profile
	if err := json.Unmarshal(raw, &prof); err != nil {
		return Profile{}, fmt.Errorf("profiles: parse %s: %w", p, err)
	}
	prof.Raw = json.RawMessage(raw)
	expanded, err := l.expandPolicies(prof.DisallowedTools)
	if err != nil {
		return Profile{}, fmt.Errorf("profiles: expand policies in %s: %w", p, err)
	}
	prof.DisallowedTools = expanded
	expandedAllowed, err := l.expandPolicies(prof.AllowedTools)
	if err != nil {
		return Profile{}, fmt.Errorf("profiles: expand policies in %s: %w", p, err)
	}
	prof.AllowedTools = expandedAllowed
	return prof, nil
}

const policyFile = "tool-policy.json"
const policyPrefix = "$include_policy:"

// expandPolicies replaces each $include_policy:<name> entry with that policy's
// tools, dropping duplicates; without a sentinel it never reads the policy file.
func (l *Loader) expandPolicies(tools []string) ([]string, error) {
	hasSentinel := false
	for _, t := range tools {
		if strings.HasPrefix(t, policyPrefix) {
			hasSentinel = true
			break
		}
	}
	if !hasSentinel {
		return tools, nil
	}

	raw, err := fs.ReadFile(l.fs, policyFile)
	if err != nil {
		return nil, fmt.Errorf("$include_policy used but %s not found: %w", policyFile, err)
	}
	var doc struct {
		Policies map[string][]string `json:"policies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", policyFile, err)
	}

	seen := make(map[string]bool, len(tools))
	result := make([]string, 0, len(tools))
	for _, t := range tools {
		if strings.HasPrefix(t, policyPrefix) {
			name := t[len(policyPrefix):]
			entries, ok := doc.Policies[name]
			if !ok {
				return nil, fmt.Errorf("unknown policy %q in $include_policy directive", name)
			}
			for _, e := range entries {
				if !seen[e] {
					seen[e] = true
					result = append(result, e)
				}
			}
		} else {
			if !seen[t] {
				seen[t] = true
				result = append(result, t)
			}
		}
	}
	return result, nil
}

// List returns the sorted basenames of the JSON files that carry a non-empty "name".
func (l *Loader) List() ([]string, error) {
	if l.fs == nil {
		return nil, nil
	}
	entries, err := fs.ReadDir(l.fs, ".")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".json") {
			continue
		}
		raw, rerr := fs.ReadFile(l.fs, n)
		if rerr != nil {
			continue
		}
		var quick struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &quick) != nil || quick.Name == "" {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".json"))
	}
	sort.Strings(out)
	return out, nil
}
