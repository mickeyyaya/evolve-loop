package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// StateMachine encodes the orchestrator lifecycle graph. It is the
// runtime authority for "is this transition legal?" and "given a
// PASS/FAIL verdict, what runs next?".
//
// The graph:
//
//	start ──┬─→ intent ──→ scout
//	        └─→ scout
//	scout ──┬─→ triage ──→ tdd
//	        └─→ tdd                  (when EVOLVE_TRIAGE_DISABLE=1)
//	tdd → build-planner → build → audit   (build-planner is skipped when EVOLVE_BUILD_PLANNER≠1)
//	audit ──┬─→ ship    (PASS or WARN — EGPS v10 accepts WARN as soft-pass)
//	        └─→ retro
//	retro ──┬─→ tdd     (RETRY per failure-adapter)
//	        ├─→ ship    (recovered)
//	        └─→ end     (BLOCK)
//	ship  → end
type StateMachine struct {
	// allowed[from] is the set of legal `to` phases; config-driven via
	// WithLegalGraph, with the literal below as the byte-identical fallback.
	allowed map[Phase]map[Phase]bool
	// specFor resolves a phase's descriptor so Next can read its verdict-branch
	// config (OnPass/OnFail). nil ⇒ Next degrades to the literal table.
	specFor func(Phase) (phasespec.PhaseSpec, bool)
	// spine is the config-declared linear successor sequence. Empty ⇒ the
	// canonical spineOrder literal.
	spine []Phase
}

// spineOrder is the canonical linear successor sequence the state machine
// walks for any phase that is not a verdict branch (audit), a control
// sentinel (retro/debugger), or the intent-independent start edge. This is
// NOT cfg.Order — cfg.Order interleaves optional insertions (spec-verify,
// tester, …) the static spine skips.
//
// audit appears so build→audit resolves, but its OWN successor is never taken
// via spineNext: Next intercepts audit in the explicit switch before the
// spine walk. end is the terminal waypoint for ship→end.
var spineOrder = []Phase{
	PhaseIntent,
	PhaseScout,
	PhaseTriage,
	PhaseTDD,
	PhaseBuildPlanner,
	PhaseBuild,
	PhaseAudit,
	PhaseShip,
	PhaseEnd,
}

// effectiveSpine is the config-declared spine (sm.spine) when present, else
// the canonical spineOrder literal.
func (sm *StateMachine) effectiveSpine() []Phase {
	if len(sm.spine) > 0 {
		return sm.spine
	}
	return spineOrder
}

// spineNext returns the phase immediately following p in the effective spine,
// and whether p is on it. A miss (sentinel, swarm-plan, or the terminal end)
// leaves the caller to handle p explicitly.
func (sm *StateMachine) spineNext(p Phase) (Phase, bool) {
	spine := sm.effectiveSpine()
	for i, sp := range spine {
		if sp == p && i+1 < len(spine) {
			return spine[i+1], true
		}
	}
	return "", false
}

// WithSpine injects the config-declared linear spine. An empty order leaves
// the SM on the canonical spineOrder literal.
func (sm *StateMachine) WithSpine(order []Phase) *StateMachine {
	sm.spine = order
	return sm
}

// spinePhasesFrom converts config phase names (registry vocabulary, e.g.
// "retrospective"/"end") to the kernel's Phase spine, denormalizing through
// phaseFromRouter. Empty/unknown names are dropped; an empty result leaves
// the SM on the canonical literal.
func spinePhasesFrom(names []string) []Phase {
	var out []Phase
	for _, n := range names {
		if p := phaseFromRouter(n); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// legalGraphFrom builds the kernel legality graph from the registry's
// config.legal_successors map. Both keys and successor names denormalize
// through phaseFromRouter (registry vocab → core.Phase, e.g.
// "retrospective"→retro), so the graph spans the sentinels (start/end/debugger)
// the registry phases[] array does not list. Returns nil for an empty map —
// the signal to WithLegalGraph to keep the literal default. Unresolvable names
// are dropped here; ValidateSafetyInvariants reports them as violations
// against the source config so the drop is never silent at load.
func legalGraphFrom(successors map[string][]string) map[Phase]map[Phase]bool {
	if len(successors) == 0 {
		return nil
	}
	graph := make(map[Phase]map[Phase]bool, len(successors))
	for fromName, tos := range successors {
		from := phaseFromRouter(fromName)
		if from == "" {
			continue
		}
		edges := make(map[Phase]bool, len(tos))
		for _, toName := range tos {
			if to := phaseFromRouter(toName); to != "" {
				edges[to] = true
			}
		}
		graph[from] = edges
	}
	return graph
}

// WithLegalGraph injects the config-driven legality graph. A nil/empty graph
// leaves the SM on its literal `allowed` default.
func (sm *StateMachine) WithLegalGraph(graph map[Phase]map[Phase]bool) *StateMachine {
	if len(graph) > 0 {
		sm.allowed = graph
	}
	return sm
}

// WithCatalog gives the StateMachine config-driven transition resolution: a
// phase whose descriptor declares on_pass/on_fail resolves its verdict branch
// from config instead of a hardcoded phase-name case. When unset, or when a
// phase declares no on_pass/on_fail, Next degrades to the exact literal table.
func (sm *StateMachine) WithCatalog(specFor func(Phase) (phasespec.PhaseSpec, bool)) *StateMachine {
	sm.specFor = specFor
	return sm
}

// NewStateMachine returns a state machine wired with the canonical
// transition table.
func NewStateMachine() *StateMachine {
	a := map[Phase]map[Phase]bool{
		PhaseStart:        {PhaseIntent: true, PhaseScout: true},
		PhaseIntent:       {PhaseScout: true},
		PhaseScout:        {PhaseTriage: true, PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTriage:       {PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTDD:          {PhaseBuildPlanner: true, PhaseBuild: true},
		PhaseBuildPlanner: {PhaseBuild: true},
		PhaseBuild:        {PhaseAudit: true},
		PhaseAudit:        {PhaseShip: true, PhaseRetro: true, PhaseTDD: true, PhaseBuild: true},
		PhaseRetro:        {PhaseShip: true, PhaseTDD: true, PhaseEnd: true, PhaseAudit: true},
		PhaseShip:         {PhaseEnd: true, PhaseDebugger: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseShip: true},
		PhaseDebugger:     {PhaseShip: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseEnd: true},
		PhaseEnd:          {},
	}
	return &StateMachine{allowed: a}
}

// CanTerminateEarly reports whether the cycle may legally END now from `from`
// — i.e. the advisor proposes a no-ship convergence cycle and there is no
// further work to evaluate. It is the SEMANTIC gate on the guarded
// scout/triage→end edges (CanTransition reports structural legality; this
// reports whether taking the edge is permitted).
func (sm *StateMachine) CanTerminateEarly(from Phase, shipPlanned bool) bool {
	if shipPlanned {
		return false
	}
	if sm.specFor != nil {
		if spec, ok := sm.specFor(from); ok && spec.EarlyExit != nil {
			return *spec.EarlyExit
		}
	}
	switch from {
	case PhaseScout, PhaseTriage:
		return true
	default:
		return false
	}
}

// CanTransition reports whether from → to is a legal edge.
func (sm *StateMachine) CanTransition(from, to Phase) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	return sm.allowed[from][to]
}

// NextFromStart returns the first phase to run, gated by intent
// requirement. The state machine encodes two legal start edges
// (start→intent and start→scout); this helper picks between them so
// callers don't need to thread cycle-state through Next().
func (sm *StateMachine) NextFromStart(intentRequired bool) Phase {
	if intentRequired {
		return PhaseIntent
	}
	return PhaseScout
}

// Next returns the verdict-driven successor of current. It only encodes
// the simplest deterministic rules — the failure-adapter is consulted
// by the orchestrator for the retro→{tdd, ship, end} branch.
func (sm *StateMachine) Next(current Phase, verdict string) (Phase, error) {
	if !current.IsValid() {
		return "", fmt.Errorf("%w: %s", ErrPhaseInvalid, current)
	}
	if sm.specFor != nil {
		if spec, ok := sm.specFor(current); ok && spec.OnPass != "" && spec.OnFail != "" {
			switch verdict {
			case VerdictPASS, VerdictWARN:
				if next := phaseFromRouter(spec.OnPass); next != "" {
					return next, nil
				}
				return "", fmt.Errorf("%w: %s on_pass %q resolves to no known phase", ErrTransitionInvalid, current, spec.OnPass)
			case VerdictFAIL:
				if next := phaseFromRouter(spec.OnFail); next != "" {
					return next, nil
				}
				return "", fmt.Errorf("%w: %s on_fail %q resolves to no known phase", ErrTransitionInvalid, current, spec.OnFail)
			default:
				return "", fmt.Errorf("%w: %s verdict %q", ErrTransitionInvalid, current, verdict)
			}
		}
	}
	switch current {
	case PhaseStart:
		return PhaseScout, nil
	case PhaseAudit:
		switch verdict {
		case VerdictPASS, VerdictWARN:
			return PhaseShip, nil
		case VerdictFAIL:
			return PhaseRetro, nil
		default:
			return "", fmt.Errorf("%w: audit verdict %q", ErrTransitionInvalid, verdict)
		}
	case PhaseDebugger, PhaseRetro:
		return PhaseEnd, nil
	case PhaseEnd:
		return "", fmt.Errorf("%w: end is terminal", ErrTransitionInvalid)
	}
	if next, ok := sm.spineNext(current); ok {
		return next, nil
	}
	return "", fmt.Errorf("%w: no successor for %s", ErrTransitionInvalid, current)
}

// mandatoryAnchorsFor is the spine-anchor order, derived entirely from
// config: the mandatory phases in the configured order.
func mandatoryAnchorsFor(cfg config.RoutingConfig) []Phase {
	var anchors []Phase
	for _, name := range effectiveOrder(cfg) {
		if !isConfiguredMandatory(cfg, name) {
			continue
		}
		if p := phaseFromRouter(name); p != "" {
			anchors = append(anchors, p)
		}
	}
	return anchors
}

// effectiveOrder is the phase sequence the floor positions anchors against:
// cfg.Order when the registry supplies one, else cfg.Mandatory. NB: distinct
// from router.effectiveOrder, whose fallback is the router's canonicalOrder.
func effectiveOrder(cfg config.RoutingConfig) []string {
	if len(cfg.Order) > 0 {
		return cfg.Order
	}
	return cfg.Mandatory
}

// SpineSatisfiedUpTo is the artifact-backed structural gate: an anchor target
// may run only if every configured-mandatory anchor ordered before it has
// produced a real handoff artifact this cycle (Audit additionally requires a
// PASS/WARN verdict). A non-anchor target is unconstrained here.
func (sm *StateMachine) SpineSatisfiedUpTo(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) bool {
	_, unsatisfied := sm.UnsatisfiedSpineAnchor(target, sig, cfg)
	return !unsatisfied
}

// UnsatisfiedSpineAnchor is SpineSatisfiedUpTo's reporter: it returns the
// first mandatory predecessor anchor of target whose handoff artifact is
// missing, and whether such an anchor exists.
func (sm *StateMachine) UnsatisfiedSpineAnchor(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) (Phase, bool) {
	anchors := mandatoryAnchorsFor(cfg)
	ti := -1
	for i, a := range anchors {
		if a == target {
			ti = i
			break
		}
	}
	if ti < 0 {
		return "", false
	}
	for i := 0; i < ti; i++ {
		if !sm.gateSatisfied(anchors[i], sig) {
			return anchors[i], true
		}
	}
	return "", false
}

// gateSatisfied reports whether anchor's artifact floor holds against the
// digest. When the catalog declares the anchor's gate thresholds
// (requires_present / verdict_in), those config values decide; otherwise the
// literal anchorArtifactPresent map is the byte-identical fallback.
func (sm *StateMachine) gateSatisfied(anchor Phase, sig router.RoutingSignals) bool {
	if sm.specFor != nil {
		if spec, ok := sm.specFor(anchor); ok && spec.Gate != nil {
			present, verdict := digestSignalFor(anchor, sig)
			if spec.Gate.RequiresPresent && !present {
				return false
			}
			if len(spec.Gate.VerdictIn) > 0 && !containsString(spec.Gate.VerdictIn, verdict) {
				return false
			}
			return true
		}
	}
	return anchorArtifactPresent(anchor, sig)
}

// digestSignalFor reads an anchor's (present, verdict) from the trusted
// on-disk signal digest. An anchor with no digest slot is treated as present.
func digestSignalFor(anchor Phase, sig router.RoutingSignals) (present bool, verdict string) {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present, ""
	case PhaseBuild:
		return sig.Build.Present, ""
	case PhaseAudit:
		return sig.Audit.Present, sig.Audit.Verdict
	}
	// Fail closed: an anchor with a config gate but no Go digest reader must
	// not silently pass — it forces the reader to be implemented.
	return false, ""
}

// anchorArtifactPresent is the literal artifact-floor map — the byte-identical
// fallback when a phase declares no config gate.
func anchorArtifactPresent(anchor Phase, sig router.RoutingSignals) bool {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present
	case PhaseBuild:
		return sig.Build.Present
	case PhaseAudit:
		return sig.Audit.Present && (sig.Audit.Verdict == VerdictPASS || sig.Audit.Verdict == VerdictWARN)
	case PhaseShip:
		return true
	}
	return true
}

// containsString reports whether s is in ss.
func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func isConfiguredMandatory(cfg config.RoutingConfig, phase string) bool {
	for _, m := range cfg.Mandatory {
		if m == phase {
			return true
		}
	}
	return false
}
