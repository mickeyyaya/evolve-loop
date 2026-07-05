// Package setup powers the `evolve setup` onboarding subcommand: the
// deterministic core behind the in-session /setup skill.
//
//   - Detect  — aggregate per-CLI auth/binary/capability (reuse bridge.Doctor +
//     capability.Inspect) + per-phase current routing (resolvellm.Resolve) +
//     per-phase constraints (profile envelope / cross-family / allowed_clis).
//   - Complete — stamp the first-run marker into state.json via a LOSSLESS
//     raw-merge (never core.State WriteState, which drops unmodeled keys like
//     expected_ship_sha).
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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
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

// --- CLI-name normalization ---

// baseCLI strips driver suffixes: claude-tmux/claude-p → claude, codex-tmux →
// codex, agy-tmux → agy.
func baseCLI(cli string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(cli), "-tmux"), "-p")
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
var abstractTiers = []string{"fast", "balanced", "deep", "top"}

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

// tierModelsFor resolves the abstract {fast,balanced,deep,top} tiers → this CLI's
// NATIVE model identifier via the bridge manifest's model_tier_map (the
// single source of truth — e.g. agy deep→Gemini 3.1 Pro (High), codex deep→
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
	// native model (e.g. agy deep→Gemini 3.1 Pro (High), codex deep→gpt-5.5), surfaced by
	// the /setup skill so per-phase routing is self-documenting; the realizer
	// resolves the same via tier_aliases.
	TierModels map[string]string `json:"tier_models,omitempty"`
}

// PhaseStatus is one phase agent's current routing + constraints. Source is
// "profile" for the profile default or "policy-pin" when a .evolve/policy.json
// pin overrides it. PinViolation is non-empty when that pin breaches the floor
// (cli ∉ allowed_clis, or model tier outside the envelope) — surfaced so the
// /setup onboarding loop can fix the offending pin before it hard-fails dispatch.
type PhaseStatus struct {
	Role        string `json:"role"`
	CurrentCLI  string `json:"current_cli,omitempty"`
	CurrentTier string `json:"current_tier,omitempty"`
	Source      string `json:"source"`
	// DefaultCLI/DefaultTier are the PROFILE defaults (profile cli +
	// model_tier_default), carried independently of any policy-pin overlay so the
	// deterministic recommender (Recommend) can compute "differs from default"
	// without re-loading the profile. Unpinned phases have these == Current*;
	// pinned phases keep the profile default here while Current* shows the pin.
	DefaultCLI      string   `json:"default_cli,omitempty"`
	DefaultTier     string   `json:"default_tier,omitempty"`
	Envelope        Envelope `json:"envelope"`
	CrossFamilyWith string   `json:"cross_family_with,omitempty"`
	AllowedCLIs     []string `json:"allowed_clis,omitempty"`
	PinViolation    string   `json:"pin_violation,omitempty"`
}

// DetectReport is the digest the /setup skill consumes.
type DetectReport struct {
	ScannedAt        string        `json:"scanned_at"`
	CLIs             []CLIStatus   `json:"clis"`
	Phases           []PhaseStatus `json:"phases"`
	SetupCompletedAt string        `json:"setup_completed_at,omitempty"`
	SetupVersion     int           `json:"setup_version,omitempty"`
	// PolicyError is non-empty when .evolve/policy.json exists but is malformed.
	// A malformed policy disables pin overlay (the floor still applies at
	// dispatch); surfaced so onboarding can fix the JSON rather than silently
	// ignore the user's pins.
	PolicyError string `json:"policy_error,omitempty"`
}

// DetectOptions configures Detect. Seams (Env/Doctor/CapTier/Now) keep the
// detection deterministic + offline in tests.
type DetectOptions struct {
	ProjectRoot string
	EvolveDir   string
	PluginRoot  string
	AdaptersDir string
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

	// Phases — current routing + profile constraints, then the user's
	// .evolve/policy.json pins overlaid (Step 9 removed llm_config.json; profiles
	// own the default CLI+tier and policy pins are the user-owned override layer
	// the /setup skill writes).
	profilesDir := filepath.Join(o.EvolveDir, "profiles")
	pol, polErr := policy.Load(filepath.Join(o.EvolveDir, "policy.json"))
	profLoader := profiles.NewFromDir(profilesDir)
	var phases []PhaseStatus
	for _, role := range Roles {
		ps := PhaseStatus{Role: role, Source: "unresolved"}
		if res, err := resolvellm.Resolve(role, resolvellm.Options{
			ProjectRoot: o.ProjectRoot, PluginRoot: o.PluginRoot, Env: env,
		}); err == nil {
			// Step 9: resolvellm emits a tier, never an exact model.
			ps.CurrentCLI, ps.CurrentTier, ps.Source = res.CLI, res.ModelTier, res.Source
		}
		if pc, ok := readProfileConstraints(profilesDir, role); ok {
			ps.Envelope, ps.CrossFamilyWith, ps.AllowedCLIs = pc.Envelope, pc.CrossFamilyWith, pc.AllowedCLIs
			ps.DefaultCLI, ps.DefaultTier = pc.DefaultCLI, pc.DefaultTier
		}
		// Overlay the policy pin (the dispatch resolver honors it absolutely) and
		// validate it against the same floor dispatch enforces, so onboarding can
		// fix a breaching pin before it hard-fails a real cycle. A malformed
		// policy (polErr != nil) is reported via dr.PolicyError below — skip the
		// overlay rather than act on a partially-parsed Policy.
		if pin, ok := pol.PinFor(role); polErr == nil && ok {
			if pin.CLI != "" {
				ps.CurrentCLI = pin.CLI
			}
			if pin.Model != "" {
				ps.CurrentTier = pin.Model
			}
			ps.Source = "policy-pin"
			if prof, err := profLoader.Get(role); err == nil {
				if verr := policy.ValidatePin(role, pin, &prof); verr != nil {
					ps.PinViolation = verr.Error()
				}
			} else {
				// A pin for a phase with no profile can't be floor-checked — say so
				// rather than show a false green (source=policy-pin, no violation).
				ps.PinViolation = fmt.Sprintf("profile %s.json not found; pin cannot be validated", role)
			}
		}
		phases = append(phases, ps)
	}

	dr := DetectReport{ScannedAt: now().UTC().Format(time.RFC3339), CLIs: clis, Phases: phases}
	if polErr != nil {
		dr.PolicyError = polErr.Error()
	}
	dr.SetupCompletedAt, dr.SetupVersion = readStateMarker(o.EvolveDir)
	return dr
}

// authMode synthesizes the auth mode. For claude the README precedence holds
// (proxy > api-key > oauth > misconfigured); other CLIs are file-subscription.
func authMode(base string, auth bridge.AuthInfo, env func(string) string) string {
	if base == "claude" {
		switch {
		case env("ANTHROPIC_BASE_URL") != "":
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

// profileConstraints is the per-phase onboarding view read from a <role>.json
// profile: the kernel-clamp guardrails (envelope/cross-family/allowed_clis) plus
// the profile DEFAULTS (cli + model_tier_default) the recommender baselines on.
type profileConstraints struct {
	Envelope        Envelope
	CrossFamilyWith string
	AllowedCLIs     []string
	DefaultCLI      string // profile.cli (raw driver name, e.g. "agy-tmux")
	DefaultTier     string // profile.model_tier_default
}

func readProfileConstraints(profilesDir, role string) (profileConstraints, bool) {
	b, err := os.ReadFile(filepath.Join(profilesDir, role+".json"))
	if err != nil {
		return profileConstraints{}, false
	}
	var doc struct {
		Envelope        Envelope `json:"model_tier_envelope"`
		CrossFamilyWith string   `json:"cross_family_with"`
		AllowedCLIs     []string `json:"allowed_clis"`
		CLI             string   `json:"cli"`
		DefaultTier     string   `json:"model_tier_default"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return profileConstraints{}, false
	}
	return profileConstraints{
		Envelope:        doc.Envelope,
		CrossFamilyWith: doc.CrossFamilyWith,
		AllowedCLIs:     doc.AllowedCLIs,
		DefaultCLI:      doc.CLI,
		DefaultTier:     doc.DefaultTier,
	}, true
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
