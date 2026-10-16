package core

import (
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func ValidateSafetyInvariants(sm *StateMachine, cfg config.RoutingConfig, cat phasespec.Catalog) []string {
	var violations []string

	for _, name := range cat.Names() {
		spec, _ := cat.Get(name)
		from := phaseFromRouter(name)
		for _, b := range []struct{ label, target string }{
			{"on_pass", spec.OnPass},
			{"on_fail", spec.OnFail},
		} {
			if b.target == "" {
				continue
			}
			to := phaseFromRouter(b.target)
			if to == "" {
				violations = append(violations, fmt.Sprintf("phase %q %s %q resolves to no known phase", name, b.label, b.target))
				continue
			}
			if sm != nil && !sm.CanTransition(from, to) {
				violations = append(violations, fmt.Sprintf("phase %q %s target %q is not a legal successor (config may only select legal edges)", name, b.label, b.target))
			}
		}
	}

	for _, n := range cfg.SpineOrder {
		if phaseFromRouter(n) == "" {
			violations = append(violations, fmt.Sprintf("spine_order phase %q resolves to no known phase", n))
		}
	}
	if sm != nil {
		spine := spinePhasesFrom(cfg.SpineOrder)
		for i := 0; i+1 < len(spine); i++ {
			if !sm.CanTransition(spine[i], spine[i+1]) {
				violations = append(violations, fmt.Sprintf("spine edge %q→%q is not a legal transition", spine[i], spine[i+1]))
			}
		}
	}

	for from, tos := range cfg.LegalSuccessors {
		if phaseFromRouter(from) == "" {
			violations = append(violations, fmt.Sprintf("legal_successors phase %q resolves to no known phase", from))
		}
		for _, to := range tos {
			if phaseFromRouter(to) == "" {
				violations = append(violations, fmt.Sprintf("legal_successors %q successor %q resolves to no known phase", from, to))
			}
		}
	}

	for _, name := range cfg.Mandatory {
		spec, ok := cat.Get(name)
		if !ok || spec.Gate == nil {
			continue
		}
		for _, v := range spec.Gate.VerdictIn {
			if v != VerdictPASS && v != VerdictWARN {
				violations = append(violations, fmt.Sprintf("mandatory phase %q gate verdict_in %q is not a shippable verdict (only PASS/WARN may gate the floor)", name, v))
			}
		}
	}

	evals := mandatoryEvaluators(cfg, cat)
	if catalogDescribesAnyMandatory(cfg, cat) && len(evals) == 0 {
		violations = append(violations, "no mandatory phase gates on a shippable verdict (PASS/WARN) — the ship floor has no mandatory evaluator; ship could proceed without a verdict gate")
	}

	if sm != nil {
		start := sm.sourceNode()
		if start == "" {
			violations = append(violations, "transition graph has no start node (every phase has an incoming edge)")
		} else {
			reach := sm.reachableFrom(start)
			anchors := mandatoryAnchorsFor(cfg)
			for _, m := range anchors {
				if !reach[m] {
					violations = append(violations, fmt.Sprintf("mandatory anchor %q is unreachable from the start node", m))
				}
			}

			if len(evals) > 0 && len(anchors) > 0 {
				sink := anchors[len(anchors)-1]
				if sm.reachableAvoiding(start, evals)[sink] {
					violations = append(violations, "the floor evaluator does not dominate the ship sink — a start→ship path bypasses every evaluator")
				}
			}
		}
	}

	return violations
}

func catalogDescribesAnyMandatory(cfg config.RoutingConfig, cat phasespec.Catalog) bool {
	for _, name := range cfg.Mandatory {
		if _, ok := cat.Get(name); ok {
			return true
		}
	}
	return false
}

func mandatoryEvaluators(cfg config.RoutingConfig, cat phasespec.Catalog) map[Phase]bool {
	evals := map[Phase]bool{}
	for _, name := range cfg.Mandatory {
		spec, ok := cat.Get(name)
		if !ok || spec.Gate == nil || len(spec.Gate.VerdictIn) == 0 {
			continue
		}
		shippable := true
		for _, v := range spec.Gate.VerdictIn {
			if v != VerdictPASS && v != VerdictWARN {
				shippable = false
				break
			}
		}
		if shippable {
			evals[phaseFromRouter(name)] = true
		}
	}
	return evals
}

func (sm *StateMachine) reachableFrom(start Phase) map[Phase]bool {
	reach := map[Phase]bool{}
	var walk func(p Phase)
	walk = func(p Phase) {
		if reach[p] {
			return
		}
		reach[p] = true
		for to := range sm.allowed[p] {
			walk(to)
		}
	}
	walk(start)
	return reach
}

func (sm *StateMachine) reachableAvoiding(start Phase, avoid map[Phase]bool) map[Phase]bool {
	reach := map[Phase]bool{}
	var walk func(p Phase)
	walk = func(p Phase) {
		if reach[p] || avoid[p] {
			return
		}
		reach[p] = true
		for to := range sm.allowed[p] {
			walk(to)
		}
	}
	walk(start)
	return reach
}

func (sm *StateMachine) sourceNode() Phase {
	hasIncoming := map[Phase]bool{}
	for _, tos := range sm.allowed {
		for to := range tos {
			hasIncoming[to] = true
		}
	}
	var candidates []Phase
	for from := range sm.allowed {
		if !hasIncoming[from] {
			candidates = append(candidates, from)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	return candidates[0]
}

func (sm *StateMachine) everyPathReaches(from, target Phase) bool {
	return sm.onlyLeadsTo(from, target, map[Phase]bool{})
}

func (sm *StateMachine) onlyLeadsTo(p, target Phase, onPath map[Phase]bool) bool {
	if onPath[p] || len(sm.allowed[p]) == 0 {
		return false
	}
	onPath[p] = true
	defer delete(onPath, p)
	for to := range sm.allowed[p] {
		if to != target && !sm.onlyLeadsTo(to, target, onPath) {
			return false
		}
	}
	return true
}
