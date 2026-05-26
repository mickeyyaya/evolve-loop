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

// RoutingConfig is the immutable, typed configuration object. Loaded once at
// the composition root, injected everywhere else.
type RoutingConfig struct {
	Stage         Stage
	Mode          Mode
	Mandatory     []string            // ordered mandatory phase names
	Conditional   map[string]CondRule // phase -> conditional-mandatory rule
	MaxInsertions int
	PhaseEnable   map[string]Enable       // phase -> enablement source
	Triggers      map[string]RoutingBlock // phase -> declarative triggers
}

// Warning is a non-fatal config diagnostic surfaced to the operator (and ledger).
type Warning struct {
	Code    string // "weak-spine" | "unknown-value" | "unknown-key"
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
	return cfg, ws
}

func defaults() RoutingConfig {
	return RoutingConfig{
		Stage:         StageOff,
		Mode:          ModeDynamicLLM,
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

func validateSpine(cfg RoutingConfig, ws *[]Warning) {
	has := func(p string) bool {
		for _, m := range cfg.Mandatory {
			if m == p {
				return true
			}
		}
		return false
	}
	var missing []string
	if !has("audit") {
		missing = append(missing, "audit")
	}
	if !has("ship") {
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
