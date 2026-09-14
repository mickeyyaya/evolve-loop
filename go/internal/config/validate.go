package config

// validate.go — the two post-resolution validators: the audit-before-ship
// spine (weak-spine, spine-order) and the inert force-enable (a phase enabled
// below advisory that the static state machine never reaches). Pure over the
// resolved value; each appends its warning with the fields a triage reads.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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
// The classic trigger is plan-review enabled via policy.json with default routing:
// plan-review only runs at Stage>=Advisory, so the enable is silently inert at
// Stage=Off AND at Stage=Shadow (per the Stage docstring, shadow computes+logs but
// the STATIC state machine still drives execution — so non-spine phases remain
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
		warn(ws, codeInertPhaseEnable,
			fmt.Sprintf("phase %q is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable", p),
			map[string]string{"phase": p, "stage": cfg.Stage.String()})
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
		warn(ws, codeWeakSpine, "mandatory_phases omits "+strings.Join(missing, "+")+" — audit-before-ship guarantee weakened",
			map[string]string{"missing": strings.Join(missing, "+")})
	}
	// The artifact-backed floor (core.SpineSatisfiedUpTo) walks the mandatory
	// anchors in their configured-order position, so a scrambled order that places
	// ship before audit would let ship's gate skip the shippable-audit check. The
	// legality graph + audit verdict branch still independently block it, but
	// surface the misordering loudly so it is never the sole guard.
	auditPos, shipPos := -1, -1
	for i, p := range cfg.Order {
		switch p {
		case "audit":
			auditPos = i
		case "ship":
			shipPos = i
		}
	}
	if auditPos >= 0 && shipPos >= 0 && auditPos > shipPos {
		warn(ws, codeSpineOrder, "phase order places ship before audit — audit must precede ship for the shippable-audit floor to gate ship",
			map[string]string{"audit_pos": strconv.Itoa(auditPos), "ship_pos": strconv.Itoa(shipPos)})
	}
}
