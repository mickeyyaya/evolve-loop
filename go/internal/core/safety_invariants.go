package core

import (
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// ValidateSafetyInvariants is the phase-agnostic load-time trust anchor. The
// legality graph and gates live in config, so the floor's non-gameability
// cannot rest on a hardcoded graph literal; this validator HARD-checks that
// the transition graph + config preserve the ship floor, quantified over the
// graph and config ROLES (mandatory anchors, verdict branches) — never
// phase-name literals — so an operator may rename any phase without
// weakening the floor. Returns human-readable violations; empty == safe.
// See ADR-0060.
func ValidateSafetyInvariants(sm *StateMachine, cfg config.RoutingConfig, cat phasespec.Catalog) []string {
	var violations []string

	// Branch targets must be known, legal successors: config selects among legal edges, never invents one.
	// See ADR-0058.
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

	// A config spine must join known phases by legal edges: Next walks it without re-checking CanTransition.
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

	// legalGraphFrom silently drops an unknown name, so an unresolvable legal_successors entry is reported here.
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

	// A mandatory phase's verdict gate may accept only shippable verdicts.
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

	// Some mandatory phase must gate on a shippable verdict, judged only when the catalog describes a mandatory phase.
	// See ADR-0060.
	evals := mandatoryEvaluators(cfg, cat)
	if catalogDescribesAnyMandatory(cfg, cat) && len(evals) == 0 {
		violations = append(violations, "no mandatory phase gates on a shippable verdict (PASS/WARN) — the ship floor has no mandatory evaluator; ship could proceed without a verdict gate")
	}

	// Every mandatory anchor must be reachable from start; a stranded anchor is a silent floor hole.
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

			// Every start→ship path must traverse a floor evaluator: ship stays unreachable with the evaluator set removed.
			// See ADR-0060.
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

// catalogDescribesAnyMandatory reports whether the catalog has an entry for at
// least one mandatory phase — i.e. the catalog is authoritative over the floor.
// It gates the floor-evaluator check so synthetic/empty test catalogs (which do
// not describe the registry's gates) are not falsely flagged.
func catalogDescribesAnyMandatory(cfg config.RoutingConfig, cat phasespec.Catalog) bool {
	for _, name := range cfg.Mandatory {
		if _, ok := cat.Get(name); ok {
			return true
		}
	}
	return false
}

// mandatoryEvaluators returns the SET of mandatory phases (as graph nodes) that
// carry an artifact gate constraining the verdict to a NON-EMPTY shippable set
// (⊆ {PASS, WARN}). These are the floor's evaluators — the phases whose verdict
// ship structurally depends on. Phase-agnostic: it inspects roles via the gate,
// never a phase name, so a renamed evaluator still satisfies the floor. The set
// keys are graph nodes (via phaseFromRouter) so the dominance walk can delete
// them directly.
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

// reachableFrom returns the set of phases reachable from start via legal
// transitions. Pure graph analysis (no phase-name literals) over the
// config-driven graph. The visited set makes it safe on the cyclic
// transition graph (ship→ship, audit→ship).
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

// reachableAvoiding returns the phases reachable from start via legal transitions
// WITHOUT entering any node in avoid (the avoided nodes are treated as deleted
// from the graph). It is the dominance primitive: if the ship sink is reachable
// while avoiding the whole evaluator set, the evaluator does not dominate ship.
// Mirrors reachableFrom; the visited set keeps it safe on the cyclic graph.
//
// If start itself is in avoid the result is empty — correct, not a missed bypass:
// every path leaves from start, so a start that is an evaluator is on every path
// and trivially dominates the sink (no violation). Seeding start unconditionally
// would instead FALSELY flag a direct start→sink edge, so the early return stands.
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
