package config

import (
	"fmt"
	"strconv"
	"strings"
)

type envDial struct {
	key   string
	apply func(cfg *RoutingConfig, v string, ws *[]Warning)
}

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

func envPhaseIO(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.PhaseIO = parseStage(v, "EVOLVE_PHASE_IO", ws)
}

func envSandbox(cfg *RoutingConfig, v string, ws *[]Warning) {
	cfg.SandboxMode = parseSandboxMode(v, cfg.SandboxMode, ws)
}

func envMandatoryPhases(cfg *RoutingConfig, v string, _ *[]Warning) {
	cfg.Mandatory = splitCSV(v)
}

func parseSandboxMode(v, current string, ws *[]Warning) string {
	switch t := strings.TrimSpace(v); t {
	case SandboxModeAuto, SandboxModeOn, SandboxModeOff:
		return t
	}
	warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_SANDBOX=%q unknown (want auto|on|off), defaulting to %q", v, current),
		map[string]string{"key": "EVOLVE_SANDBOX", "value": v, "default": current})
	return current
}

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

func envMaxOptionalInsertions(cfg *RoutingConfig, v string, ws *[]Warning) {
	n, err := strconv.Atoi(v)
	if err != nil {
		warn(ws, codeUnknownValue, fmt.Sprintf("EVOLVE_MAX_OPTIONAL_INSERTIONS=%q not an int", v),
			map[string]string{"key": "EVOLVE_MAX_OPTIONAL_INSERTIONS", "value": v})
		return
	}
	cfg.MaxInsertions = n
}
