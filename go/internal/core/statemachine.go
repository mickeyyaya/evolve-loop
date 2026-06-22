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
	// spine is the config-declared linear successor sequence (registry
	// config.spine_order, PA-DDK DDK-3). Empty ⇒ the canonical spineOrder literal
	// (the byte-identical fallback for catalog-less SMs / a registry that omits
	// it). Injected via WithSpine at the composition root.
	spine []Phase
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

// effectiveSpine is the config-declared spine (sm.spine) when present, else the
// canonical spineOrder literal — the byte-identical fallback that keeps
// catalog-less SMs and a registry omitting config.spine_order unchanged.
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

// WithSpine injects the config-declared linear spine (PA-DDK DDK-3). An empty
// order leaves the SM on the canonical spineOrder literal.
func (sm *StateMachine) WithSpine(order []Phase) *StateMachine {
	sm.spine = order
	return sm
}

// spinePhasesFrom converts config phase names (registry vocabulary, e.g.
// "retrospective"/"end") to the kernel's Phase spine, denormalizing through
// phaseFromRouter. Empty/unknown names are dropped; an empty result leaves the
// SM on the canonical literal (PA-DDK DDK-3).
func spinePhasesFrom(names []string) []Phase {
	var out []Phase
	for _, n := range names {
		if p := phaseFromRouter(n); p != "" {
			out = append(out, p)
		}
	}
	return out
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
	if next, ok := sm.spineNext(current); ok {
		return next, nil
	}
	return "", fmt.Errorf("%w: no successor for %s", ErrTransitionInvalid, current)
}

// mandatoryAnchorsFor is the spine-anchor ORDER, derived ENTIRELY from config
// (ADR-0058 S6): the mandatory phases in the configured order. No phase is a Go
// literal here — an operator sets the spine anchors purely by editing the
// registry's phase order / mandatory_phases. effectiveOrder falls back to
// cfg.Mandatory when no registry order is loaded, so the floor always has anchors.
func mandatoryAnchorsFor(cfg config.RoutingConfig) []Phase {
	var anchors []Phase
	for _, name := range effectiveOrder(cfg) {
		if !isConfiguredMandatory(cfg, name) {
			continue
		}
		// Only built-in phases carry artifact-gate semantics; an unrecognized
		// mandatory name (a user phase) maps to "" — skip it rather than seed the
		// anchor list with an empty no-op Phase.
		if p := phaseFromRouter(name); p != "" {
			anchors = append(anchors, p)
		}
	}
	return anchors
}

// effectiveOrder is the phase sequence the floor positions anchors against:
// cfg.Order when the registry supplies one, else cfg.Mandatory (so a registry-
// less SM still orders its anchors). Pure config — no hardcoded phase order.
// NB: distinct from router.effectiveOrder, whose fallback is the router's
// canonicalOrder; the floor falls back to cfg.Mandatory.
func effectiveOrder(cfg config.RoutingConfig) []string {
	if len(cfg.Order) > 0 {
		return cfg.Order
	}
	return cfg.Mandatory
}

// SpineSatisfiedUpTo is the artifact-backed structural gate: an ANCHOR target may
// run only if every configured-mandatory anchor ordered BEFORE it has produced a
// real handoff artifact this cycle (Audit additionally requires a PASS/WARN
// verdict). A non-anchor target is unconstrained here — the spine floor gates
// only the mandatory anchors; the router's plan + legality gate guard optional/
// user insertions, and CanTerminateEarly gates end.
//
// Because it keys off RoutingSignals.<X>.Present — digested from real on-disk
// handoffs — the orchestrator cannot reach Ship by merely claiming Audit passed;
// a real audit artifact with a shippable verdict must exist. This is the
// non-gameable floor that survives whichever routing Strategy is selected. The
// anchor SET and ORDER are config-driven (mandatoryAnchorsFor); only HOW a
// handoff is verified (anchorArtifactPresent) is fixed verification logic — an
// anchor with no declared check (e.g. a never-skip triage) is a no-op.
func (sm *StateMachine) SpineSatisfiedUpTo(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) bool {
	anchors := mandatoryAnchorsFor(cfg)
	ti := -1
	for i, a := range anchors {
		if a == target {
			ti = i
			break
		}
	}
	if ti < 0 {
		return true // non-anchor target: not gated by the spine floor
	}
	for i := 0; i < ti; i++ {
		if !sm.gateSatisfied(anchors[i], sig) {
			return false
		}
	}
	return true
}

// gateSatisfied reports whether anchor's artifact floor holds against the digest
// (PA-DDK DDK-4). When the catalog declares the anchor's gate THRESHOLDS
// (requires_present / verdict_in), those config values decide; otherwise the
// literal anchorArtifactPresent map is the byte-identical fallback. The digest
// (which signal proves the handoff) stays trusted Go — only the thresholds are
// config (ADR-0060).
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

// digestSignalFor reads an anchor's (present, verdict) from the trusted on-disk
// signal digest. This phase→signal mapping is the VERIFICATION layer, kept in Go
// by design (ADR-0060): config sets the gate thresholds, code reads the
// objective artifacts. An anchor with no digest slot is treated as present.
func digestSignalFor(anchor Phase, sig router.RoutingSignals) (present bool, verdict string) {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present, ""
	case PhaseBuild:
		return sig.Build.Present, ""
	case PhaseAudit:
		return sig.Audit.Present, sig.Audit.Verdict
	}
	// Fail CLOSED: an anchor with a config gate but no Go digest reader must not
	// silently pass — it forces the reader to be implemented (anti-fabrication).
	// Unconfigured anchors never reach here (they take the literal fallback).
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
		return true // ship has no pre-artifact of its own
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
