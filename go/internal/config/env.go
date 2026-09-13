package config

// env.go — the env half of resolution: the contained EVOLVE_* dials as a
// Strategy table in their declared (pinned) order. The table loop stamps the
// env var's name onto whatever a dial warned, so an env-sourced unknown-value
// names the var in fields.key while the message keeps its registry spelling.
// model_routing is CONFIG-DRIVEN via the registry only — no env dial: the
// flag-ceiling ratchet forbids adding operator env flags.

import (
	"fmt"
	"strconv"
	"strings"
)

// envDial is one contained env override: the var and what it sets.
type envDial struct {
	key   string
	apply func(cfg *RoutingConfig, v string, ws *[]Warning)
}

// envDials is the ONE list of env dials the loader reads, in the order their
// warnings appear (the kitchen-sink golden pins it).
var envDials = []envDial{
	{key: "EVOLVE_DYNAMIC_ROUTING", apply: envDynamicRouting},
	{key: "EVOLVE_ROUTING_MODE", apply: envRoutingMode},
	{key: "EVOLVE_COMMIT_EVIDENCE", apply: envCommitEvidence},
	{key: "EVOLVE_PHASE_IO", apply: envPhaseIO},
	{key: "EVOLVE_SANDBOX", apply: envSandbox},
	{key: "EVOLVE_MANDATORY_PHASES", apply: envMandatoryPhases},
	{key: "EVOLVE_CONDITIONAL_MANDATORY", apply: envConditionalMandatory},
	{key: "EVOLVE_MAX_OPTIONAL_INSERTIONS", apply: envMaxOptionalInsertions},
}

// applyEnv overlays the contained env dials (precedence: env > registry >
// default). Each dial's warnings are stamped with the var's name.
func applyEnv(cfg *RoutingConfig, env map[string]string, ws *[]Warning) {
	for _, d := range envDials {
		v := env[d.key]
		if v == "" {
			continue
		}
		from := len(*ws)
		d.apply(cfg, v, ws)
		stamp((*ws)[from:], "key", d.key)
	}
}

func envDynamicRouting(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.Stage = parseStage(v, "dynamic_routing", ws)
}

func envRoutingMode(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.Mode = parseMode(v, ws)
}

func envCommitEvidence(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.CommitEvidence = parseEvidenceStage(v, "EVOLVE_COMMIT_EVIDENCE", ws)
}

// envPhaseIO is the ADR-0050 Phase 3 unified phase-I/O rollout dial. Reuses
// parseStage (the 4-value off→shadow→advisory→enforce ladder) so a typo falls
// back to off (fail-safe), never leaving the dial in an unintended state.
// Default (no env) is enforce as of the 3.10 cutover, set in defaults(); set
// EVOLVE_PHASE_IO=off to roll back.
func envPhaseIO(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.PhaseIO = parseStage(v, "EVOLVE_PHASE_IO", ws)
}

func envSandbox(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.SandboxMode = parseSandboxMode(v, cfg.SandboxMode, ws)
}

func envMandatoryPhases(cfg *RoutingConfig, v string, _ *[]Warning) {
	cfg.Mandatory = splitCSV(v)
}

// parseSandboxMode parses EVOLVE_SANDBOX (auto|on|off, trimmed); an unknown
// word keeps the current mode and the warning names it.
func parseSandboxMode(v, current string, ws *[]Warning) string {
	switch t := strings.TrimSpace(v); t {
	case SandboxModeAuto, SandboxModeOn, SandboxModeOff:
		return t
	}
	warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_SANDBOX=%q unknown (want auto|on|off), defaulting to %q", v, current),
		map[string]string{"key": "EVOLVE_SANDBOX", "value": v, "default": current})
	return current
}

// envConditionalMandatory installs one phase:expr rule (e.g.
// tdd:cycle_size!=trivial). A value without the ':' separator or with an
// unparseable expression is ignored with a warning — never a silently
// shorter rule set.
func envConditionalMandatory(cfg *RoutingConfig, v string, ws *[]Warning) {
	phase, expr, ok := strings.Cut(v, ":")
	if !ok {
		warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_CONDITIONAL_MANDATORY=%q: missing ':' (want phase:expr)", v),
			map[string]string{"key": "EVOLVE_CONDITIONAL_MANDATORY", "value": v, "err": "missing ':'"})
		return
	}
	rule, err := parseCondRule(expr)
	if err != nil {
		warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_CONDITIONAL_MANDATORY=%q: %v", v, err),
			map[string]string{"key": "EVOLVE_CONDITIONAL_MANDATORY", "value": v, "err": err.Error()})
		return
	}
	cfg.Conditional[strings.TrimSpace(phase)] = rule
}

// envMaxOptionalInsertions overrides the optional-insertion cap; a
// non-integer is ignored with a warning.
func envMaxOptionalInsertions(cfg *RoutingConfig, v string, ws *[]Warning) {
	n, err := strconv.Atoi(v)
	if err != nil {
		warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_MAX_OPTIONAL_INSERTIONS=%q not an int", v),
			map[string]string{"key": "EVOLVE_MAX_OPTIONAL_INSERTIONS", "value": v})
		return
	}
	cfg.MaxInsertions = n
}
