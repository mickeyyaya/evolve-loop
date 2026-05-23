package core

import "fmt"

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
//	tdd → build → audit
//	audit ──┬─→ ship    (PASS or WARN — EGPS v10 accepts WARN as soft-pass)
//	        └─→ retro
//	retro ──┬─→ tdd     (RETRY per failure-adapter)
//	        ├─→ ship    (recovered)
//	        └─→ end     (BLOCK)
//	ship  → end
type StateMachine struct {
	// allowed[from] is the set of legal `to` phases.
	allowed map[Phase]map[Phase]bool
}

// NewStateMachine returns a state machine wired with the canonical
// transition table.
func NewStateMachine() *StateMachine {
	a := map[Phase]map[Phase]bool{
		PhaseStart:  {PhaseIntent: true, PhaseScout: true},
		PhaseIntent: {PhaseScout: true},
		PhaseScout:  {PhaseTriage: true, PhaseTDD: true},
		PhaseTriage: {PhaseTDD: true},
		PhaseTDD:    {PhaseBuild: true},
		PhaseBuild:  {PhaseAudit: true},
		PhaseAudit:  {PhaseShip: true, PhaseRetro: true},
		PhaseRetro:  {PhaseShip: true, PhaseTDD: true, PhaseEnd: true},
		PhaseShip:   {PhaseEnd: true},
		PhaseEnd:    {},
	}
	return &StateMachine{allowed: a}
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
	switch current {
	case PhaseStart:
		return PhaseScout, nil
	case PhaseIntent:
		return PhaseScout, nil
	case PhaseScout:
		return PhaseTriage, nil
	case PhaseTriage:
		return PhaseTDD, nil
	case PhaseTDD:
		return PhaseBuild, nil
	case PhaseBuild:
		return PhaseAudit, nil
	case PhaseAudit:
		switch verdict {
		case VerdictPASS, VerdictWARN:
			return PhaseShip, nil
		case VerdictFAIL:
			return PhaseRetro, nil
		default:
			return "", fmt.Errorf("%w: audit verdict %q", ErrTransitionInvalid, verdict)
		}
	case PhaseShip:
		return PhaseEnd, nil
	case PhaseRetro:
		// Default: failure-adapter is consulted by orchestrator. State
		// machine returns end so callers can override explicitly.
		return PhaseEnd, nil
	case PhaseEnd:
		return "", fmt.Errorf("%w: end is terminal", ErrTransitionInvalid)
	}
	return "", fmt.Errorf("%w: no successor for %s", ErrTransitionInvalid, current)
}
