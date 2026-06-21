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
// The graph (see parent plan §2 and §4 Phase 1 #9):
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
	// allowed[from] is the set of legal `to` phases. This legality graph is a
	// config-INDEPENDENT trust anchor (ADR-0058): config can only select among
	// edges it already permits, never invent one.
	allowed map[Phase]map[Phase]bool
	// specFor resolves a phase's descriptor so Next can read its verdict-branch
	// config (OnPass/OnFail). nil ⇒ Next degrades to the literal table. Injected
	// by the orchestrator (which owns the catalog) via WithCatalog.
	specFor func(Phase) (phasespec.PhaseSpec, bool)
}

// spineOrder is the canonical linear successor sequence — the mandatory-default
// spine the state machine walks for any phase that is not a verdict branch
// (audit), a control sentinel (retro/debugger), or the intent-independent start
// edge. spineNext walks it so the LINEAR transition is a data lookup, not a
// per-phase switch (ADR-0058 S5). Like the legality graph (§1) it is a config-
// INDEPENDENT trust anchor: config SELECTS among already-legal edges
// (on_pass/on_fail), it can never move the spine itself. NB: this is NOT
// cfg.Order — cfg.Order interleaves optional insertions (spec-verify, tester, …)
// the static spine skips; reproducing the literal spine needs its own SSOT.
//
// audit appears so build→audit resolves, but its OWN successor is never taken via
// spineNext: Next intercepts audit in the explicit switch (the verdict branch)
// before the spine walk. end is the terminal waypoint for ship→end.
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

// spineNext returns the phase immediately following p in spineOrder, and whether
// p is on the spine. A miss (sentinel, swarm-plan, or the terminal end) leaves
// the caller to handle p explicitly.
func spineNext(p Phase) (Phase, bool) {
	for i, sp := range spineOrder {
		if sp == p && i+1 < len(spineOrder) {
			return spineOrder[i+1], true
		}
	}
	return "", false
}

// WithCatalog gives the StateMachine config-driven transition resolution: a
// phase whose descriptor declares on_pass/on_fail resolves its verdict branch
// from config instead of a hardcoded phase-name case (ADR-0058). When unset
// (bare unit-test SMs) or when a phase declares no on_pass/on_fail, Next
// degrades to the exact literal table — so the kernel stays byte-identical for
// catalog-less orchestrators and a registry missing the fields.
func (sm *StateMachine) WithCatalog(specFor func(Phase) (phasespec.PhaseSpec, bool)) *StateMachine {
	sm.specFor = specFor
	return sm
}

// NewStateMachine returns a state machine wired with the canonical
// transition table.
func NewStateMachine() *StateMachine {
	a := map[Phase]map[Phase]bool{
		// Dynamic routing widens the spine with trivial-cycle skip edges
		// (scout/triage → build when tdd is skipped on a trivial cycle). The
		// canonical order is unchanged; these only make the skip paths LEGAL so
		// the router's enforce-mode decisions validate via CanTransition.
		PhaseStart:  {PhaseIntent: true, PhaseScout: true},
		PhaseIntent: {PhaseScout: true},
		// scout/triage → end are the guarded EARLY-EXIT edges (no-ship
		// convergence, e.g. scout found nothing to do). They are structurally
		// legal so CanTransition passes, but the SEMANTIC authority is
		// CanTerminateEarly — the orchestrator must consult it (a ship-intended
		// cycle can never take these edges). See CanTerminateEarly.
		PhaseScout:        {PhaseTriage: true, PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTriage:       {PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTDD:          {PhaseBuildPlanner: true, PhaseBuild: true},
		PhaseBuildPlanner: {PhaseBuild: true},
		PhaseBuild:        {PhaseAudit: true},
		PhaseAudit:        {PhaseShip: true, PhaseRetro: true},
		PhaseRetro:        {PhaseShip: true, PhaseTDD: true, PhaseEnd: true},
		// Ship can hand off to a recovery phase when it returns a structured
		// ShipError (advisor-recommended recovery, Component #6/#7): the
		// recovery Chain-of-Responsibility may route a precondition error to
		// re-run audit, a transient error to retry ship, or any unknown error
		// to the debugger. PhaseEnd is the success successor (and the
		// integrity-breach / recovery-exhausted abort target).
		PhaseShip: {PhaseEnd: true, PhaseDebugger: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseShip: true},
		// Debugger recovery routes: re-attempt ship, or re-run an upstream
		// phase to re-establish a stale precondition, or give up (end). Edges
		// are legal; the actual choice comes from the debug-decision the
		// orchestrator reads (decideAfterDebugger), like the retro branch.
		PhaseDebugger: {PhaseShip: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseEnd: true},
		PhaseEnd:      {},
	}
	return &StateMachine{allowed: a}
}

// CanTerminateEarly reports whether the cycle may legally END now from `from` —
// i.e. the advisor proposes a no-ship convergence cycle and there is no further
// work to evaluate. It is the SEMANTIC gate on the guarded scout/triage→end
// edges (CanTransition reports structural legality; this reports whether taking
// the edge is permitted).
//
// The invariant it defends: early-exit is ONLY ever a no-ship convergence. A
// ship-intended cycle (shipPlanned) can NEVER terminate early — it must satisfy
// the full integrity floor (build ∧ audit) and reach ship through the normal
// spine. And only the pre-build decision points (scout, triage) may terminate
// early: past build, real work exists and must be evaluated, not abandoned.
// Together these guarantee no path lands at `end` having intended to ship
// without a real, audit-bound build.
func (sm *StateMachine) CanTerminateEarly(from Phase, shipPlanned bool) bool {
	if shipPlanned {
		return false
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
	// Config-driven verdict branch (ADR-0058): a phase whose descriptor declares
	// on_pass/on_fail resolves its successor from the verdict via config, not a
	// hardcoded phase-name case. Targets are denormalized through phaseFromRouter
	// (registry vocab → core.Phase). The legality graph still gates the chosen
	// edge downstream, so config can only pick an already-legal successor. Absent
	// a catalog (bare SM) or the fields, control falls through to the literal
	// table below — byte-identical, as the transition oracle proves.
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
	// Non-linear cases the spine table cannot express, handled explicitly:
	switch current {
	case PhaseStart:
		// Intent-independent by design (NextFromStart, not Next, gates intent).
		return PhaseScout, nil
	case PhaseAudit:
		// Verdict branch — the literal fallback when no catalog declares
		// on_pass/on_fail (the config branch above handles the wired case).
		switch verdict {
		case VerdictPASS, VerdictWARN:
			return PhaseShip, nil
		case VerdictFAIL:
			return PhaseRetro, nil
		default:
			return "", fmt.Errorf("%w: audit verdict %q", ErrTransitionInvalid, verdict)
		}
	case PhaseDebugger, PhaseRetro:
		// Decision/failure-adapter driven (RESHIP / RERUN_PHASE / BLOCK and the
		// retro recovery): the orchestrator overrides via scheduledNext. Default
		// end so callers can override explicitly.
		return PhaseEnd, nil
	case PhaseEnd:
		return "", fmt.Errorf("%w: end is terminal", ErrTransitionInvalid)
	}
	// Linear spine (ADR-0058 S5): the successor is the next entry in the canonical
	// spine table — a data walk, byte-identical to the former per-phase switch.
	if next, ok := spineNext(current); ok {
		return next, nil
	}
	return "", fmt.Errorf("%w: no successor for %s", ErrTransitionInvalid, current)
}

// mandatoryAnchors is the canonical order of the spine anchors. Membership in
// the *effective* spine is decided per-config; this only fixes their order.
var mandatoryAnchors = []Phase{PhaseScout, PhaseBuild, PhaseAudit, PhaseShip}

// SpineSatisfiedUpTo is the artifact-backed structural gate: target may run only
// if every configured-mandatory anchor ordered BEFORE it has produced a real
// handoff artifact this cycle (Audit additionally requires a PASS/WARN verdict).
//
// Because it keys off RoutingSignals.<X>.Present — digested from real on-disk
// handoffs — the orchestrator cannot reach Ship by merely claiming Audit passed;
// a real audit artifact with a shippable verdict must exist. This is the
// non-gameable floor that survives whichever routing Strategy is selected.
func (sm *StateMachine) SpineSatisfiedUpTo(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) bool {
	ti := anchorIndex(target)
	if ti < 0 {
		// target is not an anchor (an optional phase): only require that any
		// anchors strictly before its nearest preceding anchor are satisfied.
		ti = precedingAnchorBound(target)
	}
	for i := 0; i < ti; i++ {
		anchor := mandatoryAnchors[i]
		if !isConfiguredMandatory(cfg, string(anchor)) {
			continue // operator removed this anchor from the mandatory set
		}
		if !anchorArtifactPresent(anchor, sig) {
			return false
		}
	}
	return true
}

// anchorIndex returns target's position in mandatoryAnchors, or -1.
func anchorIndex(target Phase) int {
	for i, a := range mandatoryAnchors {
		if a == target {
			return i
		}
	}
	return -1
}

// precedingAnchorBound maps a non-anchor target to the number of anchors that
// must already be satisfied before it can run, based on canonical position.
func precedingAnchorBound(target Phase) int {
	switch target {
	case PhaseStart, PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner:
		return 0 // before/at the build anchor — only scout precedes, handled by index 0
	case PhaseBuild:
		return 1 // scout must be present
	case PhaseRetro, PhaseEnd:
		return 3 // after audit
	default:
		return 0
	}
}

func anchorArtifactPresent(anchor Phase, sig router.RoutingSignals) bool {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present
	case PhaseBuild:
		return sig.Build.Present
	case PhaseAudit:
		return sig.Audit.Present && (sig.Audit.Verdict == VerdictPASS || sig.Audit.Verdict == VerdictWARN)
	case PhaseShip:
		return true // ship has no pre-artifact of its own
	}
	return true
}

func isConfiguredMandatory(cfg config.RoutingConfig, phase string) bool {
	for _, m := range cfg.Mandatory {
		if m == phase {
			return true
		}
	}
	return false
}
