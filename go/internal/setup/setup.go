// Package setup powers the `evolve setup` onboarding subcommand: the
// deterministic core behind the in-session /setup skill.
//
//   - Detect  — aggregate per-CLI auth/binary/capability (reuse bridge.Doctor +
//     capability.Inspect) + per-phase current routing (resolvellm.Resolve) +
//     per-phase constraints (profile envelope / cross-family / allowed_clis).
//   - Validate — clamp a proposed llm_config.json against the integrity floor:
//     tier ∈ profile envelope (ERROR), cli ∈ allowed_clis (ERROR),
//     builder-family ≠ auditor-family (WARN advisory; ERROR under --strict).
//   - Complete — stamp the first-run marker into state.json via a LOSSLESS
//     raw-merge (never core.State WriteState, which drops unmodeled keys like
//     expected_ship_sha).
//
// The skill proposes; Validate is the kernel clamp — "model proposes, kernel
// disposes", the same pattern the routing engine uses.
package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// Version is the onboarding schema version stamped by Complete.
const Version = 1

// Roles are the phase agents the setup flow configures, in pipeline order.
// Each is resolvable by resolvellm.Resolve and has a <role>.json profile.
var Roles = []string{
	"intent", "scout", "triage", "plan-reviewer", "tdd-engineer",
	"build-planner", "builder", "tester", "auditor", "orchestrator",
	"retrospective", "memo",
}

// --- tier + family normalization (canonical vocabulary: fast/balanced/deep,
// matching profiles' model_tier_default + model_tier_envelope and each
// manifest's model_tier_map. The Anthropic-named legacy tokens haiku/
// sonnet/opus are still recognized for one release for backward compat
// with operator-installed v1 config files.) ---

// tierRank maps a canonical tier (fast/balanced/deep) — or a legacy alias
// (haiku/sonnet/opus) — or an exact model string — to 1/2/3; 0 = unclassifiable
// (envelope check is skipped for rank 0). The substring fallbacks at the
// bottom handle full model identifiers like "claude-haiku-4-5-20251001".
func tierRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fast", "haiku":
		return 1
	case "balanced", "sonnet":
		return 2
	case "deep", "opus":
		return 3
	}
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "haiku"):
		return 1
	case strings.Contains(l, "sonnet"):
		return 2
	case strings.Contains(l, "opus"):
		return 3
	}
	return 0
}

// baseCLI strips driver suffixes: claude-tmux/claude-p → claude, codex-tmux →
// codex, agy-tmux → agy.
func baseCLI(cli string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(cli), "-tmux"), "-p")
}

// cliFamily maps a CLI to its model-vendor family (the cross-family axis:
// builder-family must differ from auditor-family for adversarial integrity).
func cliFamily(cli string) string {
	switch baseCLI(cli) {
	case "claude":
		return "anthropic"
	case "codex":
		return "openai"
	case "gemini", "agy":
		return "google"
	}
	return baseCLI(cli)
}

// capManifest maps a base CLI to its capability-manifest stem (agy's manifest
// file is antigravity.capabilities.json).
func capManifest(base string) string {
	if base == "agy" {
		return "antigravity"
	}
	return base
}

// abstractTiers is the canonical vocabulary used end-to-end in the
// pipeline: profile model_tier_default, profile model_tier_envelope, the
// model_tier_map keys in each manifest, and resolvellm sentinel defaults.
// One source of truth, no cross-pollination.
var abstractTiers = []string{"fast", "balanced", "deep"}

// familyDriverManifest maps a base CLI family to the bridge manifest that
// carries its model_tier_map (the interactive -tmux drivers are the multi-
// CLI defaults; their maps match the headless variants).
func familyDriverManifest(base string) string {
	switch base {
	case "claude":
		return "claude-tmux"
	case "codex":
		return "codex-tmux"
	case "agy":
		return "agy-tmux"
	}
	return base
}

// tierModelsFor resolves the abstract {fast,balanced,deep} tiers → this CLI's
// NATIVE model identifier via the bridge manifest's model_tier_map (the
// single source of truth — e.g. agy fast→gemini-3.5-flash, codex deep→
// gpt-5.5, claude balanced→sonnet). When the manifest declares no entry
// for an abstract tier, the abstract name passes through as the model
// identifier (identity fallback — useful for legacy manifests still on the
// v1 schema during the deprecation window). Honors operator manifest
// overrides via bridge.LoadManifest.
func tierModelsFor(base string) map[string]string {
	man, err := bridge.LoadManifest(familyDriverManifest(base))
	out := make(map[string]string, len(abstractTiers))
	for _, tier := range abstractTiers {
		model := tier // identity fallback if manifest missing the entry
		if err == nil {
			if v, ok := man.ModelTierMap[tier]; ok && v != "" {
				model = v
			}
		}
		out[tier] = model
	}
	return out
}

// --- Detect ---

// Envelope mirrors a profile's model_tier_envelope (fast/balanced/deep).
type Envelope struct {
	Min     string `json:"min,omitempty"`
	Default string `json:"default,omitempty"`
	Max     string `json:"max,omitempty"`
}

// CLIStatus is one detected CLI family's onboarding-relevant status.
type CLIStatus struct {
	CLI              string   `json:"cli"` // base family: claude|codex|gemini|agy
	BinaryPresent    bool     `json:"binary_present"`
	BinaryPath       string   `json:"binary_path,omitempty"`
	AuthConfigured   bool     `json:"auth_configured"`
	AuthMode         string   `json:"auth_mode"` // SUBSCRIPTION_OAUTH|API_KEY|CUSTOM_PROXY|SUBSCRIPTION|MISCONFIGURED
	SubscriptionType string   `json:"subscription_type,omitempty"`
	CapabilityTier   string   `json:"capability_tier"` // full|delegated|n/a
	Verdict          string   `json:"verdict"`         // ready|warning|blocked
	EnvWarnings      []string `json:"env_warnings,omitempty"`
	// TierModels maps each abstract tier (fast|balanced|deep) to THIS CLI's
	// native model (e.g. agy→gemini-3.5-flash, codex deep→gpt-5.5). The
	// /setup skill writes these into llm_config.model so the config is
	// self-documenting; the realizer resolves the same via tier_aliases.
	TierModels map[string]string `json:"tier_models,omitempty"`
}

// PhaseStatus is one phase agent's current routing + constraints.
type PhaseStatus struct {
	Role            string   `json:"role"`
	CurrentCLI      string   `json:"current_cli,omitempty"`
	CurrentModel    string   `json:"current_model,omitempty"`
	CurrentTier     string   `json:"current_tier,omitempty"`
	Source          string   `json:"source"`
	Envelope        Envelope `json:"envelope"`
	CrossFamilyWith string   `json:"cross_family_with,omitempty"`
	AllowedCLIs     []string `json:"allowed_clis,omitempty"`
}

// DetectReport is the digest the /setup skill consumes.
type DetectReport struct {
	ScannedAt        string        `json:"scanned_at"`
	CLIs             []CLIStatus   `json:"clis"`
	Phases           []PhaseStatus `json:"phases"`
	SetupCompletedAt string        `json:"setup_completed_at,omitempty"`
	SetupVersion     int           `json:"setup_version,omitempty"`
}

// DetectOptions configures Detect. Seams (Env/Doctor/CapTier/Now) keep the
// detection deterministic + offline in tests.
type DetectOptions struct {
	ProjectRoot string
	EvolveDir   string
	PluginRoot  string
	AdaptersDir string
	ConfigPath  string // default <EvolveDir>/llm_config.json
	Env         func(string) string
	Now         func() time.Time
	Doctor      func(ctx context.Context) bridge.DoctorReport // default: bridge.NewEngine(Deps{}).Doctor(ctx,"",false)
	CapTier     func(base string) string                      // default: capability.Inspect under AdaptersDir
}

// Detect assembles the onboarding digest.
func Detect(ctx context.Context, o DetectOptions) DetectReport {
	env := o.Env
	if env == nil {
		env = os.Getenv
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	doctorFn := o.Doctor
	if doctorFn == nil {
		doctorFn = func(ctx context.Context) bridge.DoctorReport {
			rep, _ := bridge.NewEngine(bridge.Deps{}).Doctor(ctx, "", false)
			return rep
		}
	}
	capFn := o.CapTier
	if capFn == nil {
		capFn = func(base string) string { return capTierFromManifest(o.AdaptersDir, base) }
	}

	// CLIs — group bridge.Doctor's per-driver rows by base family.
	rep := doctorFn(ctx)
	seen := map[string]bool{}
	var clis []CLIStatus
	for _, r := range rep.Results {
		b := baseCLI(r.CLI)
		if seen[b] {
			continue
		}
		seen[b] = true
		cs := CLIStatus{
			CLI:              b,
			BinaryPresent:    r.Binary.Present,
			BinaryPath:       r.Binary.Path,
			AuthConfigured:   r.Auth.Configured,
			AuthMode:         authMode(b, r.Auth, env),
			SubscriptionType: r.Auth.SubscriptionType,
			Verdict:          r.Verdict,
			EnvWarnings:      r.EnvWarnings,
		}
		if r.Binary.Present {
			cs.CapabilityTier = capFn(b)
		} else {
			cs.CapabilityTier = "n/a"
		}
		cs.TierModels = tierModelsFor(b)
		clis = append(clis, cs)
	}
	sort.Slice(clis, func(i, j int) bool { return clis[i].CLI < clis[j].CLI })

	// Phases — current routing + profile constraints. (Step 9: llm_config
	// removed; resolvellm reads the profile directly.)
	profilesDir := filepath.Join(o.EvolveDir, "profiles")
	var phases []PhaseStatus
	for _, role := range Roles {
		ps := PhaseStatus{Role: role, Source: "unresolved"}
		if res, err := resolvellm.Resolve(role, resolvellm.Options{
			ProjectRoot: o.ProjectRoot, PluginRoot: o.PluginRoot, Env: env,
		}); err == nil {
			// Step 9: resolvellm no longer emits an exact Model (always a tier);
			// CurrentModel stays empty (report-schema field, pruned in 9b).
			ps.CurrentCLI, ps.CurrentTier, ps.Source = res.CLI, res.ModelTier, res.Source
		}
		if envlp, fam, allowed, ok := readProfileConstraints(profilesDir, role); ok {
			ps.Envelope, ps.CrossFamilyWith, ps.AllowedCLIs = envlp, fam, allowed
		}
		phases = append(phases, ps)
	}

	dr := DetectReport{ScannedAt: now().UTC().Format(time.RFC3339), CLIs: clis, Phases: phases}
	dr.SetupCompletedAt, dr.SetupVersion = readStateMarker(o.EvolveDir)
	return dr
}

// authMode synthesizes the auth mode. For claude the README precedence holds
// (proxy > api-key > oauth > misconfigured); other CLIs are file-subscription.
func authMode(base string, auth bridge.AuthInfo, env func(string) string) string {
	if base == "claude" {
		switch {
		case env("EVOLVE_ANTHROPIC_BASE_URL") != "" || env("ANTHROPIC_BASE_URL") != "":
			return "CUSTOM_PROXY"
		case env("ANTHROPIC_API_KEY") != "":
			return "API_KEY"
		case auth.Configured:
			return "SUBSCRIPTION_OAUTH"
		default:
			return "MISCONFIGURED"
		}
	}
	if auth.Configured {
		return "SUBSCRIPTION"
	}
	return "MISCONFIGURED"
}

func capTierFromManifest(adaptersDir, base string) string {
	if adaptersDir == "" {
		return "unknown"
	}
	insp, err := capability.Inspect(adaptersDir, capManifest(base))
	if err != nil {
		return "unknown"
	}
	if insp.Manifest.BudgetNative && insp.Manifest.PermissionScoping {
		return "full"
	}
	return "delegated"
}

func readProfileConstraints(profilesDir, role string) (Envelope, string, []string, bool) {
	b, err := os.ReadFile(filepath.Join(profilesDir, role+".json"))
	if err != nil {
		return Envelope{}, "", nil, false
	}
	var doc struct {
		Envelope        Envelope `json:"model_tier_envelope"`
		CrossFamilyWith string   `json:"cross_family_with"`
		AllowedCLIs     []string `json:"allowed_clis"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return Envelope{}, "", nil, false
	}
	return doc.Envelope, doc.CrossFamilyWith, doc.AllowedCLIs, true
}

func readStateMarker(evolveDir string) (string, int) {
	b, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		return "", 0
	}
	var m struct {
		SetupCompletedAt string `json:"setupCompletedAt"`
		SetupVersion     int    `json:"setupVersion"`
	}
	_ = json.Unmarshal(b, &m)
	return m.SetupCompletedAt, m.SetupVersion
}

// --- Validate ---

// Violation is one clamp finding. Severity "error" → exit 2; "warn" → advisory.
type Violation struct {
	Role     string `json:"role"`
	Kind     string `json:"kind"`     // envelope|allowed_cli|cross_family
	Severity string `json:"severity"` // error|warn
	Message  string `json:"message"`
}

// ValidateReport is the clamp result. OK = no error-severity violations.
type ValidateReport struct {
	Config     string      `json:"config"`
	Violations []Violation `json:"violations"`
	OK         bool        `json:"ok"`
}

// ValidateOptions configures Validate.
type ValidateOptions struct {
	ConfigPath  string
	ProfilesDir string // default <EvolveDir>/profiles
	EvolveDir   string
	Strict      bool // when true, cross-family same-family is an error (not a warn)
}

type cfgPhase struct {
	CLI       string `json:"cli"`
	Tier      string `json:"tier"`
	Model     string `json:"model"`
	ModelTier string `json:"model_tier"`
}

// effTier returns the phase's effective tier signal (tier > model_tier > model).
func (p cfgPhase) effTier() string {
	if p.Tier != "" {
		return p.Tier
	}
	if p.ModelTier != "" {
		return p.ModelTier
	}
	return p.Model
}

// Validate clamps a proposed llm_config.json against the integrity floor.
func Validate(o ValidateOptions) (ValidateReport, error) {
	rep := ValidateReport{Config: o.ConfigPath, OK: true}
	raw, err := os.ReadFile(o.ConfigPath)
	if err != nil {
		return rep, fmt.Errorf("setup validate: read config: %w", err)
	}
	var cfg struct {
		Phases map[string]cfgPhase `json:"phases"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return rep, fmt.Errorf("setup validate: parse config: %w", err)
	}
	profilesDir := o.ProfilesDir
	if profilesDir == "" {
		profilesDir = filepath.Join(o.EvolveDir, "profiles")
	}

	// Sort roles for deterministic violation order.
	roles := make([]string, 0, len(cfg.Phases))
	for r := range cfg.Phases {
		roles = append(roles, r)
	}
	sort.Strings(roles)

	for _, role := range roles {
		ph := cfg.Phases[role]
		env, _, allowed, ok := readProfileConstraints(profilesDir, role)
		if !ok {
			continue // no profile constraints to enforce
		}
		// Envelope bound (ERROR).
		rank := tierRank(ph.effTier())
		minR, maxR := tierRank(env.Min), tierRank(env.Max)
		if rank > 0 && minR > 0 && maxR > 0 && (rank < minR || rank > maxR) {
			rep.Violations = append(rep.Violations, Violation{
				Role: role, Kind: "envelope", Severity: "error",
				Message: fmt.Sprintf("tier %q (rank %d) outside envelope [%s..%s]", ph.effTier(), rank, env.Min, env.Max),
			})
			rep.OK = false
		}
		// allowed_clis (ERROR).
		if len(allowed) > 0 && !contains(allowed, "all") && ph.CLI != "" && !contains(allowed, baseCLI(ph.CLI)) {
			rep.Violations = append(rep.Violations, Violation{
				Role: role, Kind: "allowed_cli", Severity: "error",
				Message: fmt.Sprintf("cli %q not in allowed_clis %v", baseCLI(ph.CLI), allowed),
			})
			rep.OK = false
		}
	}

	// Cross-family: builder ≠ auditor family. WARN by default (all-Claude is a
	// legitimate fallback per the cycle-100 recovery + dynamic-model-routing.md
	// advisory posture); ERROR only under --strict.
	if b, okB := cfg.Phases["builder"]; okB {
		if a, okA := cfg.Phases["auditor"]; okA && b.CLI != "" && a.CLI != "" && cliFamily(b.CLI) == cliFamily(a.CLI) {
			sev := "warn"
			if o.Strict {
				sev = "error"
				rep.OK = false
			}
			rep.Violations = append(rep.Violations, Violation{
				Role: "builder+auditor", Kind: "cross_family", Severity: sev,
				Message: fmt.Sprintf("builder and auditor share family %q — adversarial-audit integrity is weakened (set different-family CLIs when available)", cliFamily(b.CLI)),
			})
		}
	}
	return rep, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// --- Complete ---

// CompleteOptions configures Complete.
type CompleteOptions struct {
	EvolveDir string
	Now       func() time.Time
}

// Complete stamps setupCompletedAt + setupVersion into state.json via a
// LOSSLESS raw-merge — preserving unmodeled keys (e.g. expected_ship_sha) that
// core.State's WriteState would drop.
func Complete(o CompleteOptions) (string, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(o.EvolveDir, 0o755); err != nil {
		return "", fmt.Errorf("setup complete: mkdir: %w", err)
	}
	path := filepath.Join(o.EvolveDir, "state.json")

	obj := map[string]json.RawMessage{}
	if b, err := os.ReadFile(path); err == nil {
		if uerr := json.Unmarshal(b, &obj); uerr != nil {
			return "", fmt.Errorf("setup complete: state.json is malformed (%w); refusing to clobber", uerr)
		}
	}
	stamp := now().UTC().Format(time.RFC3339)
	tsRaw, _ := json.Marshal(stamp)
	verRaw, _ := json.Marshal(Version)
	obj["setupCompletedAt"] = tsRaw
	obj["setupVersion"] = verRaw

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("setup complete: marshal: %w", err)
	}
	out = append(out, '\n')
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", fmt.Errorf("setup complete: write temp: %w", err)
	}
	defer func() { _ = os.Remove(tmp) }() // best-effort cleanup; no-op once the rename succeeds
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("setup complete: atomic rename: %w", err)
	}
	return stamp, nil
}
