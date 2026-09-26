package interaction

// CorrectionInput holds the caller-supplied facts NextCorrection decides from; it reads no filesystem or pane.
type CorrectionInput struct {
	Phase, Workspace, Worktree string
	// Violation is the review gate's summarized rejection reason.
	Violation string
	// NamedREPL reports a named tmux session preserved through the review gate, which live_fix needs.
	NamedREPL bool
	// Busy reports the preserved pane is mid-turn; a busy agent is never interrupted.
	Busy bool
	// DecisionID correlates every rung of this correction decision in the ledger.
	DecisionID string
	// RungBudget is the remaining budget per rung; the caller decrements it, NextCorrection never does.
	RungBudget map[string]int
}

// CorrectionAction is one ladder decision; an empty Rung means every rung is exhausted.
type CorrectionAction struct {
	Rung, Reason string
}

// Ladder rungs, cheapest first; the values are the Event.Rung vocabulary.
const (
	RungSalvage    = "salvage"
	RungLiveFix    = "live_fix"
	RungRedispatch = "redispatch"
)

// Salvage and ladder outcome results, extending the Result* vocabulary.
const (
	// ResultFoundButInvalid: salvage relocated a candidate but the destination failed verification.
	ResultFoundButInvalid = "found_but_invalid"
	// ResultNotFound: no salvageable candidate existed.
	ResultNotFound = "not_found"
	// ResultWouldAct: shadow stage; the rung was selected and logged but not executed.
	ResultWouldAct = "would_act"
)

// NextCorrection picks the cheapest rung with remaining budget whose preconditions hold.
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
