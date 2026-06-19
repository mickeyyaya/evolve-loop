package interaction

// correction.go — ADR-0045 I2: the graduated correction ladder's DECISION
// function. Contract rejection used to have exactly one tool — full
// re-dispatch (cycle-265 burned two for a misplaced-but-valid file that a
// `mv` would have fixed in milliseconds). NextCorrection is a Chain of
// Responsibility over the repair rungs, cheapest first:
//
//	rung 1  salvage     — deterministic relocate-then-verify; NO agent involved
//	rung 2  live_fix    — one templated instruction into the phase's OWN
//	                      preserved named REPL (idle only — never touch a
//	                      working agent)
//	rung 3  redispatch  — today's fresh-REPL correction, evidence-enriched
//
// PURE by design (the recovery-package discipline): the ladder decision
// consumes only caller-supplied facts — no filesystem, no pane reads — so the
// order-is-load-bearing property is unit-testable without an orchestrator.
// EXECUTING a rung (and stage-gating that execution) is the caller's job.
//
// Deviation from the design sketch, with reason: CorrectionInput.Violation is
// a plain string, not deliverable.Violation — the deliverable package imports
// core (it implements core.DeliverableReviewer), so a leaf importing it would
// cycle. Same move recovery made for verdicts: identifiers cross as strings.
type CorrectionInput struct {
	Phase, Workspace, Worktree string
	// Violation is the review gate's summarized rejection reason.
	Violation string
	// NamedREPL reports that the phase ran on a NAMED tmux session preserved
	// through the review gate (claude-tmux only at v1) — rung 2's structural
	// precondition. False ⇒ rung 2 is skipped (there is no REPL to fix).
	NamedREPL bool
	// Busy reports the preserved pane is mid-turn. A busy agent is never
	// interrupted (ADR-0045 §5) — rung 2 requires idle.
	Busy bool
	// DecisionID correlates every rung of this one correction decision in
	// the I1 ledger (so a salvage outcome links to the re-dispatch it averted).
	DecisionID string
	// RungBudget is the remaining budget per rung (salvage:1, live_fix:1,
	// redispatch:configured correction limit). The caller decrements on
	// execution; NextCorrection never mutates it (pure).
	RungBudget map[string]int
}

// CorrectionAction is one ladder decision. Rung "" means every rung is
// exhausted — the caller aborts the cycle exactly as today. Reason justifies
// the choice (the ADR-0044 every-decision-justified invariant).
type CorrectionAction struct {
	Rung, Reason string
}

// Ladder rungs, cheapest-first. The values are the Event.Rung vocabulary.
const (
	RungSalvage    = "salvage"
	RungLiveFix    = "live_fix"
	RungRedispatch = "redispatch"
)

// Salvage / ladder outcome results (extends the Result* vocabulary).
const (
	// ResultFoundButInvalid: salvage located and relocated a candidate, but
	// the DESTINATION failed verification — falls through to re-dispatch,
	// which carries the path as kernel evidence.
	ResultFoundButInvalid = "found_but_invalid"
	// ResultNotFound: no salvageable candidate existed.
	ResultNotFound = "not_found"
	// ResultWouldAct: shadow stage — the rung was selected and logged but
	// deliberately not executed (the §10 soak signal).
	ResultWouldAct = "would_act"
)

// NextCorrection picks the cheapest rung with remaining budget whose
// structural preconditions hold. Order is load-bearing: salvage → live_fix →
// redispatch (§8 TestNextCorrection_OrderIsLoadBearing).
func NextCorrection(in CorrectionInput) CorrectionAction {
	if in.RungBudget[RungSalvage] > 0 {
		return CorrectionAction{
			Rung:   RungSalvage,
			Reason: "cheapest rung: relocate-then-verify the contracted artifact, no agent involved",
		}
	}
	if in.RungBudget[RungLiveFix] > 0 && in.NamedREPL && !in.Busy {
		return CorrectionAction{
			Rung:   RungLiveFix,
			Reason: "phase's own REPL is preserved and idle: one templated fix beats a full re-dispatch",
		}
	}
	if in.RungBudget[RungRedispatch] > 0 {
		reason := "fresh evidence-enriched re-dispatch"
		switch {
		case in.RungBudget[RungLiveFix] > 0 && !in.NamedREPL:
			reason += " (no preserved named session for a live fix)"
		case in.RungBudget[RungLiveFix] > 0 && in.Busy:
			reason += " (pane busy — a working agent is never interrupted)"
		}
		return CorrectionAction{Rung: RungRedispatch, Reason: reason}
	}
	return CorrectionAction{Reason: "every rung exhausted — abort the cycle as today"}
}
