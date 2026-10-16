package interaction

type CorrectionInput struct {
	Phase, Workspace, Worktree string
	Violation                  string
	Repairable                 bool
	NamedREPL                  bool
	Busy                       bool
	DecisionID                 string
	RungBudget                 map[string]int
}

type CorrectionAction struct {
	Rung, Reason string
}

const (
	RungSalvage    = "salvage"
	RungRecover    = "recover"
	RungLiveFix    = "live_fix"
	RungRedispatch = "redispatch"
)

const (
	ResultFoundButInvalid = "found_but_invalid"
	ResultNotFound        = "not_found"
	ResultWouldAct        = "would_act"
)

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
	if in.RungBudget[RungRecover] > 0 && in.Repairable {
		return CorrectionAction{
			Rung:   RungRecover,
			Reason: "the recovery agent restructures the deliverable around the phase's logic; the phase is not re-run",
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
