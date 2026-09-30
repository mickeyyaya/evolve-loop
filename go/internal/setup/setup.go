// Package setup powers `evolve setup` onboarding: CLI and routing detection,
// preset recommendation, policy pin application and the first-run marker.
// See docs/architecture/packages/internal-setup.md.
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

const Version = 1

var Roles = []string{
	"intent", "scout", "triage", "plan-reviewer", "tdd-engineer",
	"build-planner", "builder", "tester", "auditor", "orchestrator",
	"retrospective", "memo",
}

func baseCLI(cli string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(cli), "-tmux"), "-p")
}

func capManifest(base string) string {
	if base == "agy" {
		return "antigravity"
	}
	return base
}

var abstractTiers = []string{"fast", "balanced", "deep", "top"}

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

func tierModelsFor(base string) map[string]string {
	man, err := bridge.LoadManifest(familyDriverManifest(base))
	out := make(map[string]string, len(abstractTiers))
	for _, tier := range abstractTiers {
		model := tier
		if err == nil {
			if v, ok := man.ModelTierMap[tier]; ok && v != "" {
				model = v
			}
		}
		out[tier] = model
	}
	return out
}

type Envelope struct {
	Min     string `json:"min,omitempty"`
	Default string `json:"default,omitempty"`
	Max     string `json:"max,omitempty"`
}

type CLIStatus struct {
	CLI              string            `json:"cli"`
	BinaryPresent    bool              `json:"binary_present"`
	BinaryPath       string            `json:"binary_path,omitempty"`
	AuthConfigured   bool              `json:"auth_configured"`
	AuthMode         string            `json:"auth_mode"`
	SubscriptionType string            `json:"subscription_type,omitempty"`
	CapabilityTier   string            `json:"capability_tier"`
	Verdict          string            `json:"verdict"`
	EnvWarnings      []string          `json:"env_warnings,omitempty"`
	TierModels       map[string]string `json:"tier_models,omitempty"`
}

type PhaseStatus struct {
	Role            string   `json:"role"`
	CurrentCLI      string   `json:"current_cli,omitempty"`
	CurrentTier     string   `json:"current_tier,omitempty"`
	Source          string   `json:"source"`
	DefaultCLI      string   `json:"default_cli,omitempty"`
	DefaultTier     string   `json:"default_tier,omitempty"`
	Envelope        Envelope `json:"envelope"`
	CrossFamilyWith string   `json:"cross_family_with,omitempty"`
	AllowedCLIs     []string `json:"allowed_clis,omitempty"`
	PinViolation    string   `json:"pin_violation,omitempty"`
}

type DetectReport struct {
	ScannedAt        string        `json:"scanned_at"`
	CLIs             []CLIStatus   `json:"clis"`
	Phases           []PhaseStatus `json:"phases"`
	SetupCompletedAt string        `json:"setup_completed_at,omitempty"`
	SetupVersion     int           `json:"setup_version,omitempty"`
	PolicyError      string        `json:"policy_error,omitempty"`
}

type DetectOptions struct {
	ProjectRoot string
	EvolveDir   string
	PluginRoot  string
	AdaptersDir string
	Env         func(string) string
	Now         func() time.Time
	Doctor      func(ctx context.Context) bridge.DoctorReport
	CapTier     func(base string) string
}

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

	clis := detectCLIs(doctorFn(ctx), capFn, env)

	pol, polErr := policy.Load(filepath.Join(o.EvolveDir, "policy.json"))
	phases := detectPhases(o, env, pol, polErr)

	dr := DetectReport{ScannedAt: now().UTC().Format(time.RFC3339), CLIs: clis, Phases: phases}
	if polErr != nil {
		dr.PolicyError = polErr.Error()
	}
	dr.SetupCompletedAt, dr.SetupVersion = readStateMarker(o.EvolveDir)
	return dr
}

func detectCLIs(rep bridge.DoctorReport, capFn func(string) string, env func(string) string) []CLIStatus {
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
	return clis
}

func detectPhases(o DetectOptions, env func(string) string, pol policy.Policy, polErr error) []PhaseStatus {
	profilesDir := filepath.Join(o.EvolveDir, "profiles")
	profLoader := profiles.NewFromDir(profilesDir)
	var phases []PhaseStatus
	for _, role := range Roles {
		ps := PhaseStatus{Role: role, Source: "unresolved"}
		if res, err := resolvellm.Resolve(role, resolvellm.Options{
			ProjectRoot: o.ProjectRoot, PluginRoot: o.PluginRoot, Env: env,
		}); err == nil {
			ps.CurrentCLI, ps.CurrentTier, ps.Source = res.CLI, res.ModelTier, res.Source
		}
		if pc, ok := readProfileConstraints(profilesDir, role); ok {
			ps.Envelope, ps.CrossFamilyWith, ps.AllowedCLIs = pc.Envelope, pc.CrossFamilyWith, pc.AllowedCLIs
			ps.DefaultCLI, ps.DefaultTier = pc.DefaultCLI, pc.DefaultTier
		}
		if pin, ok := pol.PinFor(role); polErr == nil && ok {
			ps = withPolicyPin(ps, pin, profLoader)
		}
		phases = append(phases, ps)
	}
	return phases
}

func withPolicyPin(ps PhaseStatus, pin policy.Pin, profLoader *profiles.Loader) PhaseStatus {
	role := ps.Role
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
		ps.PinViolation = fmt.Sprintf("profile %s.json not found; pin cannot be validated", role)
	}
	return ps
}

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

type profileConstraints struct {
	Envelope        Envelope
	CrossFamilyWith string
	AllowedCLIs     []string
	DefaultCLI      string
	DefaultTier     string
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

type CompleteOptions struct {
	EvolveDir string
	Now       func() time.Time
}

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
	defer func() { _ = os.Remove(tmp) }()
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("setup complete: atomic rename: %w", err)
	}
	return stamp, nil
}
