// Package config is the single composition-root loader for evolve-loop's
// dynamic-routing configuration. It is the ONLY place that reads routing
// env vars + the central registry file; every downstream consumer receives
// the immutable RoutingConfig by injection and never calls os.Getenv.
//
// Leaf package by design (imports only stdlib): like internal/failureadapter,
// it must NOT import internal/core, so that core.Orchestrator can import it
// without a cycle. Phase identifiers cross the boundary as plain strings;
// core converts to/from core.Phase at the call site.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Stage is the dynamic-routing rollout stage (shadow → advisory → enforce).
type Stage int

const (
	StageOff      Stage = iota // legacy: static state machine drives, router off
	StageShadow                // router computes + logs, static still drives
	StageAdvisory              // router drives the optional surface; spine static
	StageEnforce               // router drives, clamped by the kernel
)

func (s Stage) String() string {
	switch s {
	case StageShadow:
		return "shadow"
	case StageAdvisory:
		return "advisory"
	case StageEnforce:
		return "enforce"
	default:
		return "0"
	}
}

// Mode selects the routing brain (Strategy). Default is DynamicLLM (locked decision).
type Mode int

const (
	ModeDynamicLLM   Mode = iota // LLM proposes, kernel clamps (default)
	ModeStaticPreset             // deterministic: triggers + spine only, no LLM
)

func (m Mode) String() string {
	if m == ModeStaticPreset {
		return "static"
	}
	return "llm"
}

// Enable is the per-phase enablement decision source.
type Enable int

const (
	EnableContent Enable = iota // decided by routing triggers (Specification)
	EnableOn                    // force-run
	EnableOff                   // force-skip
)

func (e Enable) String() string {
	switch e {
	case EnableOn:
		return "on"
	case EnableOff:
		return "off"
	default:
		return "content"
	}
}

// CondRule is a parsed conditional-mandatory predicate, e.g. cycle_size != trivial.
type CondRule struct {
	Field string
	Op    string
	Value string
}

// Condition is one declarative routing-trigger clause (a Specification), held
// as data from the registry. Evaluation lives in package router (it needs the
// digested signals); config only parses and carries it.
type Condition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// RoutingBlock is the per-phase declarative trigger set.
type RoutingBlock struct {
	InsertWhen []Condition `json:"insert_when"`
	SkipWhen   []Condition `json:"skip_when"`
}

// RolloutStages groups the three independent rollout-axis dials, each gating a
// subsystem's Off→Shadow→Enforce migration. They share no logic, but all three
// are composition-root *views* of an env-driven signal the subprocess reads
// directly. Embedded anonymously in RoutingConfig, so every cfg.CommitEvidence
// / cfg.ReviewGate / cfg.SandboxMode access is unchanged via field promotion.
type RolloutStages struct {
	// CommitEvidence is the ADR-0027 commit-as-evidence rollout stage:
	// StageOff (legacy path-poll, byte-identical), StageShadow (git-evidence
	// computed + logged, artifact authoritative), StageEnforce (git-evidence
	// authoritative, phases commit, kernel relaxed). StageAdvisory is not used
	// for this axis. The bridge driver reads EVOLVE_COMMIT_EVIDENCE from env
	// directly (it is a subprocess); this field is the orchestrator's view.
	CommitEvidence Stage
	// ReviewGate is Workstream E2's per-phase review-gate rollout stage:
	//   StageOff      — orchestrator uses noopReviewer (every non-SKIPPED
	//                   verdict approved). Byte-identical to pre-E2.
	//   StageShadow   — deterministic reviewer runs but is log-only.
	//   StageEnforce  — deterministic + (future) LLM reviewer authoritative;
	//                   reject aborts the cycle.
	// StageAdvisory is not used for this axis (no advisory-intermediate). The
	// orchestrator owns the stage interpretation; this field is the
	// composition-root view, exactly like CommitEvidence.
	ReviewGate Stage
	// EvalGate is the structural inter-phase eval-gate rollout stage
	// (internal/evalgate): StageOff — no eval gates (orchestrator keeps the
	// noopReviewer; byte-identical); StageShadow — Gate A (scout eval-file
	// materialization) + Gate B (tdd predicate-quality) run + log but always
	// approve; StageEnforce — a CERTAIN violation (a stat'd-missing eval file
	// or a definite tautology) aborts the cycle. The gates fail OPEN on any
	// ambiguity, so enforce-default never false-blocks a healthy cycle. Set
	// from EVOLVE_EVAL_GATE via applyEnv; default StageEnforce.
	EvalGate Stage
	// SandboxMode controls OS-level sandbox wrapping for source-writing phases
	// (Workstream B — cycle-119 cross-CLI trust bypass). Values:
	//   "auto" (default) — wrap when nested-claude is NOT detected and the
	//                       host's sandbox binary (sandbox-exec / bwrap) is
	//                       present; degrade unwrapped otherwise.
	//   "on"             — always wrap when the binary is available; WARN
	//                       loudly (no fallback) when it isn't.
	//   "off"            — never wrap. Operator-only emergency hatch; the
	//                       trust kernel is then Claude-PreToolUse-only.
	//
	// PRECEDENCE NOTE: the bridge subprocess reads EVOLVE_SANDBOX from its
	// own env chain (deps.Env / os.Getenv), which is the actual signal. This
	// field is the COMPOSITION-ROOT view — set from the same env var by
	// applyEnv so operators auditing the loaded config can see the effective
	// mode. Mirrors the CommitEvidence pattern (also env-direct on the
	// subprocess hot path). Setting this field in code without also propagating
	// EVOLVE_SANDBOX into the bridge's env map has no effect.
	SandboxMode string
}

// RoutingConfig is the immutable, typed configuration object. Loaded once at
// the composition root, injected everywhere else.
type RoutingConfig struct {
	Stage Stage
	Mode  Mode
	// RolloutStages embeds CommitEvidence / ReviewGate / SandboxMode — the
	// three subsystem-migration dials, promoted so existing field access is
	// unchanged (see RolloutStages).
	RolloutStages
	Mandatory     []string            // ordered mandatory phase names
	Conditional   map[string]CondRule // phase -> conditional-mandatory rule
	MaxInsertions int
	PhaseEnable   map[string]Enable       // phase -> enablement source
	Triggers      map[string]RoutingBlock // phase -> declarative triggers
	// Order is the linear phase sequence the router walks, in registry order.
	// Empty ⇒ the router falls back to its built-in canonicalOrder (so a config
	// loaded without a registry stays byte-identical to pre-Order behavior).
	// The composition root may splice user phases into this slice.
	Order []string
}

// Sandbox mode string constants — exported so the bridge + tests can match
// without sprinkling magic strings.
const (
	SandboxModeAuto = "auto"
	SandboxModeOn   = "on"
	SandboxModeOff  = "off"
)

// Warning is a non-fatal config diagnostic surfaced to the operator (and ledger).
type Warning struct {
	Code    string // "weak-spine" | "unknown-value" | "unknown-key" | "inert-phase-enable"
	Message string
}

// legacyEnableFlags maps a legacy per-phase env flag to (phase, valueWhenSet).
// Absorbing them here keeps the flags out of the phase code (no os.Getenv in
// triage/buildplanner/scout/audit/retro after the PhasePolicy refactor).
type legacyFlag struct {
	phase    string
	whenOne  Enable // value when env == "1"
	whenZero Enable // value when env == "0"
}

var legacyFlags = map[string]legacyFlag{
	"EVOLVE_REQUIRE_INTENT": {"intent", EnableOn, EnableContent},
	// EVOLVE_TRIAGE_DISABLE: =1 disables triage; =0 explicitly enables it
	// (legacy default is on, so =0 must map to On, NOT Content — Content with
	// no trigger would wrongly skip).
	"EVOLVE_TRIAGE_DISABLE": {"triage", EnableOff, EnableOn},
	"EVOLVE_PLAN_REVIEW":    {"plan-review", EnableOn, EnableOff},
	// EVOLVE_TEST_PHASE_ENABLED is the real runtime flag the tdd phase reads
	// (=1 on, =0 off); the registry's enable_var metadata historically said
	// EVOLVE_TDD_PHASE, which never matched the phase code. config.Load is now
	// the single interpreter, so it binds the flag the phase actually honors.
	"EVOLVE_TEST_PHASE_ENABLED":         {"tdd", EnableOn, EnableOff},
	"EVOLVE_BUILD_PLANNER":              {"build-planner", EnableOn, EnableOff},
	"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": {"retrospective", EnableOff, EnableContent},
	// Swarm planner (ADR-0032) — opt-in like build-planner. EVOLVE_SWARM_STAGE
	// (shadow|advisory|enforce) is the rollout dial; any non-empty value other
	// than off/shadow enables the planner phase. config.Load maps the legacy =1
	// form here; the orchestrator reads the stage for dispatch behavior.
	"EVOLVE_SWARM_PLANNER": {"swarm-plan", EnableOn, EnableOff},
}

// registryDoc is the subset of phase-registry.json this loader reads.
type registryDoc struct {
	Config struct {
		DynamicRouting        string            `json:"dynamic_routing"`
		RoutingMode           string            `json:"routing_mode"`
		MandatoryPhases       []string          `json:"mandatory_phases"`
		ConditionalMandatory  map[string]string `json:"conditional_mandatory"`
		MaxOptionalInsertions *int              `json:"max_optional_insertions"`
	} `json:"config"`
	Phases []struct {
		Name     string        `json:"name"`
		Optional bool          `json:"optional"`
		Enabled  string        `json:"enabled"`
		Routing  *RoutingBlock `json:"routing"`
	} `json:"phases"`
}

// Load resolves the effective RoutingConfig. Precedence: env override >
// registry file > built-in default. env is injected (not read from the
// process) so the loader stays testable and is the sole contained env site.
func Load(registryPath string, env map[string]string) (RoutingConfig, []Warning) {
	var ws []Warning

	cfg := defaults()

	if env["EVOLVE_USE_PHASE_REGISTRY"] != "0" {
		if doc, ok := readRegistry(registryPath); ok {
			applyRegistry(&cfg, doc, &ws)
		}
	}

	applyEnv(&cfg, env, &ws)

	validateSpine(cfg, &ws)
	validateInertEnables(cfg, &ws)
	return cfg, ws
}

// staticSpinePhases is the set of phases the legacy state machine drives as
// agent runs (excluding the start/end sentinels). When Stage==StageOff the
// router is off and ONLY these phases get a turn — so a PhaseEnable[p]=On for
// any other phase is silently inert. Encoded as a local set rather than
// imported from core because config is a leaf package; the
// TestStaticSpineMatchesStateMachine cross-package contract test pins this
// against the actual state machine's edge map.
var staticSpinePhases = map[string]struct{}{
	"intent":        {},
	"scout":         {},
	"triage":        {},
	"tdd":           {},
	"build-planner": {},
	"build":         {},
	"audit":         {},
	"ship":          {},
	"retro":         {},
}

// validateInertEnables warns when PhaseEnable[p]=EnableOn but p is neither
// mandatory, in the static spine, nor reachable via the router (Stage<Advisory).
// The classic trigger is EVOLVE_PLAN_REVIEW=1 with default routing: plan-review
// only runs at Stage>=Advisory, so the enable is silently inert at Stage=Off
// AND at Stage=Shadow (per the Stage docstring, shadow computes+logs but the
// STATIC state machine still drives execution — so non-spine phases remain
// unreachable). Surfacing this prevents the operator-confusion failure mode
// from cycle 120.
func validateInertEnables(cfg RoutingConfig, ws *[]Warning) {
	if cfg.Stage >= StageAdvisory {
		return // router drives; enable is effective
	}
	// Sort for deterministic warning order — map iteration is randomized.
	phases := make([]string, 0, len(cfg.PhaseEnable))
	for p := range cfg.PhaseEnable {
		phases = append(phases, p)
	}
	sort.Strings(phases)
	for _, p := range phases {
		if cfg.PhaseEnable[p] != EnableOn {
			continue
		}
		if containsPhase(cfg.Mandatory, p) {
			continue
		}
		if _, inSpine := staticSpinePhases[p]; inSpine {
			continue
		}
		*ws = append(*ws, Warning{"inert-phase-enable",
			fmt.Sprintf("phase %q is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable", p),
		})
	}
}

func defaults() RoutingConfig {
	return RoutingConfig{
		// Dynamic routing is DEFAULT-ON (Component #7): the advisor drives phase
		// selection every cycle, with the integrity floor (ClampPlanToFloor +
		// SpineSatisfiedUpTo) — not a flag — protecting the ship guarantee.
		// EVOLVE_DYNAMIC_ROUTING still overrides (e.g. =off for the legacy static
		// path). Flipped from StageOff after the advisory mode soaked since
		// cycle-108.
		Stage:         StageAdvisory,
		Mode:          ModeDynamicLLM,
		RolloutStages: RolloutStages{CommitEvidence: StageOff, SandboxMode: SandboxModeAuto, EvalGate: StageEnforce},
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		Conditional:   map[string]CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		// Legacy phase-enable defaults, so PhasePolicy reproduces pre-routing
		// behavior even when the registry file is absent (e.g. tests): triage
		// and tdd run by default; build-planner is opt-in (shadow). These are
		// the floor the registry `enabled` field and env flags override.
		PhaseEnable: map[string]Enable{
			"triage":        EnableOn,
			"tdd":           EnableOn,
			"build-planner": EnableOff,
			"swarm-plan":    EnableOff,
		},
		Triggers: map[string]RoutingBlock{},
	}
}

func readRegistry(path string) (registryDoc, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return registryDoc{}, false
	}
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return registryDoc{}, false
	}
	return doc, true
}

func applyRegistry(cfg *RoutingConfig, doc registryDoc, ws *[]Warning) {
	c := doc.Config
	if c.DynamicRouting != "" {
		cfg.Stage = parseStage(c.DynamicRouting, ws)
	}
	if c.RoutingMode != "" {
		cfg.Mode = parseMode(c.RoutingMode, ws)
	}
	if len(c.MandatoryPhases) > 0 {
		cfg.Mandatory = c.MandatoryPhases
	}
	if c.MaxOptionalInsertions != nil {
		cfg.MaxInsertions = *c.MaxOptionalInsertions
	}
	for phase, expr := range c.ConditionalMandatory {
		if rule, err := parseCondRule(expr); err == nil {
			cfg.Conditional[phase] = rule
		} else {
			*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("conditional_mandatory[%s]=%q: %v", phase, expr, err)})
		}
	}
	for _, p := range doc.Phases {
		if p.Name == "" {
			continue
		}
		cfg.Order = append(cfg.Order, p.Name)
		if p.Enabled != "" {
			cfg.PhaseEnable[p.Name] = parseEnable(p.Enabled, ws)
		}
		if p.Routing != nil {
			cfg.Triggers[p.Name] = *p.Routing
		}
	}
}

func applyEnv(cfg *RoutingConfig, env map[string]string, ws *[]Warning) {
	if v := env["EVOLVE_DYNAMIC_ROUTING"]; v != "" {
		cfg.Stage = parseStage(v, ws)
	}
	if v := env["EVOLVE_ROUTING_MODE"]; v != "" {
		cfg.Mode = parseMode(v, ws)
	}
	if v := env["EVOLVE_COMMIT_EVIDENCE"]; v != "" {
		cfg.CommitEvidence = parseEvidenceStage(v, "EVOLVE_COMMIT_EVIDENCE", ws)
	}
	if v := env["EVOLVE_REVIEW_GATE"]; v != "" {
		// Same off/shadow/enforce trichotomy as CommitEvidence — no advisory
		// intermediate. Reuses parseEvidenceStage to share the warning text +
		// fallback (typo defaults to off, never silently enables a kill-path).
		cfg.ReviewGate = parseEvidenceStage(v, "EVOLVE_REVIEW_GATE", ws)
	}
	if v := env["EVOLVE_EVAL_GATE"]; v != "" {
		// Structural eval gates (internal/evalgate). Same off/shadow/enforce
		// trichotomy; reuses parseEvidenceStage so a typo defaults to off
		// rather than silently enabling a kill-path. Default (no env) is
		// enforce, set in defaults().
		cfg.EvalGate = parseEvidenceStage(v, "EVOLVE_EVAL_GATE", ws)
	}
	if v := env["EVOLVE_SANDBOX"]; v != "" {
		switch strings.TrimSpace(v) {
		case SandboxModeAuto, SandboxModeOn, SandboxModeOff:
			cfg.SandboxMode = strings.TrimSpace(v)
		default:
			*ws = append(*ws, Warning{"unknown-value",
				fmt.Sprintf("EVOLVE_SANDBOX=%q unknown (want auto|on|off), defaulting to %q", v, cfg.SandboxMode)})
		}
	}
	if v := env["EVOLVE_MANDATORY_PHASES"]; v != "" {
		cfg.Mandatory = splitCSV(v)
	}
	if v := env["EVOLVE_CONDITIONAL_MANDATORY"]; v != "" {
		// format: phase:expr  e.g. tdd:cycle_size!=trivial
		if phase, expr, ok := strings.Cut(v, ":"); ok {
			if rule, err := parseCondRule(expr); err == nil {
				cfg.Conditional[strings.TrimSpace(phase)] = rule
			} else {
				*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("EVOLVE_CONDITIONAL_MANDATORY=%q: %v", v, err)})
			}
		}
	}
	if v := env["EVOLVE_MAX_OPTIONAL_INSERTIONS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxInsertions = n
		} else {
			*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("EVOLVE_MAX_OPTIONAL_INSERTIONS=%q not an int", v)})
		}
	}
	// Legacy per-phase enable flags absorbed here (kept out of phase code).
	for flag, lf := range legacyFlags {
		switch env[flag] {
		case "1":
			cfg.PhaseEnable[lf.phase] = lf.whenOne
		case "0":
			cfg.PhaseEnable[lf.phase] = lf.whenZero
		}
	}
}

// containsPhase reports whether slice contains p.
func containsPhase(slice []string, p string) bool {
	for _, s := range slice {
		if s == p {
			return true
		}
	}
	return false
}

func validateSpine(cfg RoutingConfig, ws *[]Warning) {
	var missing []string
	if !containsPhase(cfg.Mandatory, "audit") {
		missing = append(missing, "audit")
	}
	if !containsPhase(cfg.Mandatory, "ship") {
		missing = append(missing, "ship")
	}
	if len(missing) > 0 {
		*ws = append(*ws, Warning{
			Code:    "weak-spine",
			Message: "mandatory_phases omits " + strings.Join(missing, "+") + " — audit-before-ship guarantee weakened",
		})
	}
}

func parseStage(v string, ws *[]Warning) Stage {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff
	case "shadow":
		return StageShadow
	case "advisory":
		return StageAdvisory
	case "enforce":
		return StageEnforce
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("dynamic_routing=%q unknown, defaulting to off", v)})
		return StageOff
	}
}

// parseEvidenceStage parses an off/shadow/enforce dial. Unlike parseStage it has
// no "advisory" middle state — these axes are compute-and-log (shadow) vs act
// (enforce). Any unknown value defaults to off with a warning (a typo must never
// silently enable a kill-path). Shared by EVOLVE_COMMIT_EVIDENCE / _REVIEW_GATE /
// _EVAL_GATE; varName names the offending env var in the warning.
func parseEvidenceStage(v, varName string, ws *[]Warning) Stage {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff
	case "shadow":
		return StageShadow
	case "enforce":
		return StageEnforce
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("%s=%q unknown (want off|shadow|enforce), defaulting to off", varName, v)})
		return StageOff
	}
}

func parseMode(v string, ws *[]Warning) Mode {
	switch strings.TrimSpace(v) {
	case "llm", "dynamic", "dynamic-llm":
		return ModeDynamicLLM
	case "static", "static-preset", "preset":
		return ModeStaticPreset
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("routing_mode=%q unknown, defaulting to llm", v)})
		return ModeDynamicLLM
	}
}

func parseEnable(v string, ws *[]Warning) Enable {
	switch strings.TrimSpace(v) {
	case "on":
		return EnableOn
	case "off":
		return EnableOff
	case "content":
		return EnableContent
	default:
		*ws = append(*ws, Warning{"unknown-value", fmt.Sprintf("enabled=%q unknown, defaulting to content", v)})
		return EnableContent
	}
}

// parseCondRule parses "field<op>value" where op is one of != == >= <= > <.
// Tolerates surrounding whitespace. Two-char ops are matched before one-char.
func parseCondRule(expr string) (CondRule, error) {
	for _, op := range []string{"!=", "==", ">=", "<="} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+2:]),
			}, nil
		}
	}
	for _, op := range []string{">", "<"} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+1:]),
			}, nil
		}
	}
	return CondRule{}, fmt.Errorf("no comparison operator in %q", expr)
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
