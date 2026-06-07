// Package profiles loads .evolve/profiles/*.json agent permission
// profiles. These pin every load-bearing constraint for an agent
// subprocess: the CLI it runs against, the model tier, the allowed
// and disallowed tools, the sandbox configuration, the budget envelope,
// and parallel-eligibility (single-writer invariant).
//
// The loader is fs.FS-backed so the same impl serves tests
// (fstest.MapFS), the dev override (os.DirFS), and the Phase 3
// vendored embed.FS — no API churn at the consumer site.
package profiles

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Profile mirrors the .evolve/profiles/<name>.json schema. The
// load-bearing fields are typed; everything else (including leading-
// underscore informational keys like `_comment`) survives in Raw so
// callers can extract un-modeled fields via a second json.Unmarshal.
type Profile struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	CLI         string   `json:"cli"`
	AllowedCLIs []string `json:"allowed_clis,omitempty"`
	// CLIFallback is the ordered list of alternate CLIs the runner tries when
	// the primary CLI fails with one of CLIFallbackOnExit codes (Workstream
	// G — "any CLI can run any phase"). Each entry MUST be a registered
	// driver name (e.g. "claude-tmux", "agy-tmux"). Empty/absent = no
	// fallback = the legacy single-CLI behavior. Chain is tried in order;
	// the FIRST CLI that boots and returns a non-trigger exit wins.
	CLIFallback []string `json:"cli_fallback,omitempty"`
	// CLIFallbackOnExit enumerates the bridge exit codes that trigger
	// fallback (Workstream G; default extended in cycle-122 Fix 2).
	// Defaults to [80, 81, 124, 127] when nil/empty:
	//   80  = ExitREPLBootTimeout (the *-tmux REPL never showed its prompt)
	//   81  = ExitArtifactTimeout (bridge artifact-timeout — added in cycle-122)
	//   124 = coreutils timeout(1) exit code (defensive; if a wrapper uses it)
	//   127 = ExitMissingBinary  (the CLI binary isn't on PATH)
	// Operators can extend per-agent (e.g. add 2 ExitSafetyGate) for an
	// even more aggressive policy, OR shrink to [80, 127] for the
	// production-strict posture where 81 should surface to the
	// failure-adapter rather than retry. CLI failures NOT in this list
	// still hard-fail — a legitimate FAIL verdict never silently routes
	// to a different CLI. See bridge/exitcodes.go for the canonical exit
	// numbers and runner/cli_chain.go:defaultFallbackOnExit for the live
	// default per code.
	CLIFallbackOnExit  []int              `json:"cli_fallback_on_exit,omitempty"`
	ModelTierDefault   string             `json:"model_tier_default"`
	ModelTierEnvelope  *ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	ModelTierOverrides map[string]string  `json:"model_tier_overrides,omitempty"`
	AllowedTools       []string           `json:"allowed_tools,omitempty"`
	DisallowedTools    []string           `json:"disallowed_tools,omitempty"`
	MaxTurns           int                `json:"max_turns,omitempty"`
	MaxBudgetUSD       float64            `json:"max_budget_usd,omitempty"`
	BudgetTiers        map[string]float64 `json:"budget_tiers,omitempty"`
	ParallelEligible   bool               `json:"parallel_eligible,omitempty"`
	OutputArtifact     string             `json:"output_artifact,omitempty"`
	ResearchQuota      map[string]int     `json:"research_quota,omitempty"`
	Sandbox            *SandboxConfig     `json:"sandbox,omitempty"`
	EffortLevel        string             `json:"effort_level,omitempty"`
	AddDir             []string           `json:"add_dir,omitempty"`
	PermissionMode     string             `json:"permission_mode,omitempty"`
	StreamOutput       bool               `json:"stream_output,omitempty"`
	StopCriterion      string             `json:"stop_criterion,omitempty"`
	TurnBudgetHint     int                `json:"turn_budget_hint,omitempty"`
	GeneratedFrom      string             `json:"generated_from,omitempty"`
	// SystemPrompt / SystemPromptFile carry per-agent system-level rules
	// prepended to the prompt at launch (facet B). SystemPromptFile is read
	// relative to the profile dir when not absolute; SystemPrompt wins if both
	// are set.
	SystemPrompt     string `json:"system_prompt,omitempty"`
	SystemPromptFile string `json:"system_prompt_file,omitempty"`
	// Raw retains the on-disk bytes for callers needing un-modeled
	// fields (e.g., parallel_subtasks, context_anchors). Populated by
	// the loader; not part of the JSON schema.
	Raw json.RawMessage `json:"-"`
}

// ModelTierEnvelope is the {min, default, max} sub-structure used by
// profiles that constrain LLM tier escalation (e.g., triage.json).
type ModelTierEnvelope struct {
	Min     string `json:"min,omitempty"`
	Default string `json:"default,omitempty"`
	Max     string `json:"max,omitempty"`
}

// SandboxConfig is the typed shape of profile.sandbox.
type SandboxConfig struct {
	Enabled       bool     `json:"enabled,omitempty"`
	ReadOnlyRepo  bool     `json:"read_only_repo,omitempty"`
	WriteSubpaths []string `json:"write_subpaths,omitempty"`
	DenySubpaths  []string `json:"deny_subpaths,omitempty"`
	AllowNetwork  bool     `json:"allow_network,omitempty"`
}

// Loader resolves profile names to parsed Profile values.
//
// A zero loader is valid; every Get returns fs.ErrNotExist.
type Loader struct {
	fs fs.FS
}

// NewFromFS constructs a Loader backed by an arbitrary fs.FS. Pass nil
// to get the zero loader.
func NewFromFS(fsys fs.FS) *Loader { return &Loader{fs: fsys} }

// NewFromDir constructs a Loader rooted at the given directory. Empty
// path returns the zero loader.
func NewFromDir(dir string) *Loader {
	if dir == "" {
		return &Loader{}
	}
	return &Loader{fs: os.DirFS(dir)}
}

// Get reads <name>.json, parses it into a Profile, and populates Raw.
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
	// Raw keeps the ORIGINAL bytes — any $include_policy sentinels in it are
	// unexpanded. Callers extracting tool lists must use the typed fields
	// below, which carry the expanded (effective) sets.
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

// expandPolicies replaces "$include_policy:<name>" sentinels in tools with the
// entries from tool-policy.json. Duplicates are removed; sentinel order preserved.
// Returns tools unchanged when no sentinel is present (no policy file needed).
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

// List enumerates profile names (without .json extension), sorted.
// Non-JSON files (e.g., AGENTS.md, README.txt) and non-profile JSON
// files (e.g., tool-policy.json which has no "name" field) are excluded.
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
		// Skip non-profile JSON files (e.g., tool-policy.json) — profiles
		// must have a non-empty "name" field.
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
