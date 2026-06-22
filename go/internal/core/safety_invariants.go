package core

import (
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// ValidateSafetyInvariants is the phase-agnostic load-time trust anchor
// (ADR-0060). As the transition kernel becomes data-driven (PA-DDK), the
// legality graph and gates move into config; the floor's non-gameability can no
// longer rest on a hardcoded graph literal. This validator replaces it: it
// HARD-checks that the transition graph + config preserve the ship floor,
// quantified over the graph and config ROLES (mandatory anchors, verdict
// branches) — never phase-name literals — so an operator may rename any phase
// without weakening the floor. Returns human-readable violations; empty == safe.
//
// DDK-1 lands the two invariants checkable before the graph/gates are
// config-driven; DDK-4/DDK-5 extend it with the artifact-gate and
// path-dominance invariants as those fields go live, so each check lands BEFORE
// its corresponding config-flip and the floor is never unguarded.
func ValidateSafetyInvariants(sm *StateMachine, cfg config.RoutingConfig, cat phasespec.Catalog) []string {
	var violations []string

	// I8 — branch-target legality: a phase's verdict-branch targets (on_pass /
	// on_fail) must resolve to a known phase AND be a legal successor. Config may
	// only SELECT among already-legal edges, never invent one (ADR-0058 §1, now
	// enforced as a data check rather than by a hardcoded graph).
	for _, name := range cat.Names() {
		spec, ok := cat.Get(name)
		if !ok {
			continue
		}
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

	// I9 — anchor reachability: every configured-mandatory anchor must be
	// reachable from the start node. A stranded anchor cannot gate the floor, so
	// a config that marks an unreachable phase mandatory is a silent floor hole.
	// A graph with no start node at all is itself a structural violation.
	if sm != nil {
		start := sm.sourceNode()
		if start == "" {
			violations = append(violations, "transition graph has no start node (every phase has an incoming edge)")
		} else {
			reach := sm.reachableFrom(start)
			for _, m := range mandatoryAnchorsFor(cfg) {
				if !reach[m] {
					violations = append(violations, fmt.Sprintf("mandatory anchor %q is unreachable from the start node", m))
				}
			}
		}
	}

	return violations
}

// reachableFrom returns the set of phases reachable from start via legal
// transitions. Pure graph analysis (no phase-name literals) — it tracks the
// config-driven graph once DDK-5 lands. The visited set makes it safe on the
// cyclic transition graph (ship→ship, audit→ship).
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

// sourceNode returns the graph's start node — a node that is never a transition
// TARGET (in-degree 0). Identified structurally so a renamed start phase is
// still found. When several candidates exist the lowest-sorted is returned for
// determinism; "" means none (a structural violation the caller reports).
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
