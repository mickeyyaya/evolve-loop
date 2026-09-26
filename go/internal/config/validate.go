package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// staticSpinePhases are the agent phases the static state machine drives. It is a local copy
// because config cannot import core; TestStaticSpineMatchesStateMachine pins it.
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

// validateInertEnables warns on a force-enabled phase that is neither mandatory nor in the static
// spine while the router is below advisory: the static state machine drives there, so it never runs.
func validateInertEnables(cfg RoutingConfig, ws *[]Warning) {
	if cfg.Stage >= StageAdvisory {
		return
	}
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
	// The spine floor positions anchors by configured order, so ship before audit would skip the
	// shippable-audit check; the legality graph still blocks it, but the order must not be the only guard.
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
