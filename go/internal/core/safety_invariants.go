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

	// Spine-edge legality (PA-DDK DDK-3): a config-declared spine (cfg.SpineOrder)
	// must resolve to known phases and every consecutive edge must be a legal
	// transition — a spine cannot route around an anchor via an illegal jump
	// (scout→ship), since Next walks the spine without re-checking CanTransition.
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

	// Legal-successors resolvability (PA-DDK DDK-5 hardening): a config legality
	// graph (config.legal_successors) must name only known phases on BOTH sides.
	// legalGraphFrom silently drops an unresolvable name, so a typo would degrade
	// the graph with no load error — report it loudly (mirrors the on_pass/on_fail
	// "no known phase" check). Sentinels start/end/debugger resolve via phaseFromRouter.
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

	// Floor-gate verdict safety (PA-DDK DDK-4): a mandatory phase whose artifact
	// gate constrains the verdict may only accept SHIPPABLE verdicts (PASS/WARN).
	// This stops a config from weakening the floor to ship a FAILed evaluation.
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

	// Floor-evaluator existence (PA-DDK DDK-5 hardening — the operative half of
	// ADR-0060 §4 "F⊆M"): the ship floor is only real if SOME mandatory phase
	// gates on a shippable verdict — a mandatory EVALUATOR must exist. Without it
	// an operator can drop the evaluator from mandatory_phases and
	// SpineSatisfiedUpTo admits ship with no verdict gate (the runtime anchor goes
	// inert — see TestSpineSatisfiedUpTo_ConfigurableMandatoryWeakensGate). Phase-
	// agnostic: quantified over mandatory phases' gates, never a phase name. The
	// floor-gate check above forbids a NON-shippable mandatory gate; this forbids
	// the ABSENCE of one (and a presence-only gate with empty verdict_in, which a
	// FAIL verdict would otherwise slip through).
	//
	// Only judged when the catalog is AUTHORITATIVE over the floor — it describes
	// at least one mandatory phase. The real composition root always passes the
	// full registry catalog, and a production tamper (dropping the evaluator from
	// mandatory) leaves the other mandatory phases described, so the check still
	// fires. A synthetic/empty catalog (unit tests of orchestration mechanics)
	// cannot describe the evaluator's gate, so judging it would false-positive.
	if catalogDescribesAnyMandatory(cfg, cat) && !hasMandatoryShippableEvaluator(cfg, cat) {
		violations = append(violations, "no mandatory phase gates on a shippable verdict (PASS/WARN) — the ship floor has no mandatory evaluator; ship could proceed without a verdict gate")
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

// hasMandatoryShippableEvaluator reports whether some mandatory phase carries an
// artifact gate that constrains the verdict to a NON-EMPTY shippable set
// (⊆ {PASS, WARN}). That phase is the floor's evaluator — the one whose verdict
// ship structurally depends on. Phase-agnostic: it inspects roles via the gate,
// never a phase name, so a renamed evaluator still satisfies the floor.
func hasMandatoryShippableEvaluator(cfg config.RoutingConfig, cat phasespec.Catalog) bool {
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
			return true
		}
	}
	return false
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
