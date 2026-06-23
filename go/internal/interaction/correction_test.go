package interaction_test

// ADR-0045 I2 — the pure correction-ladder decision (§8 RED tests).

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

func fullBudget(redispatch int) map[string]int {
	return map[string]int{
		interaction.RungSalvage:    1,
		interaction.RungLiveFix:    1,
		interaction.RungRedispatch: redispatch,
	}
}

// TestNextCorrection_OrderIsLoadBearing — cheapest repair first: with every
// budget available (and rung-2 preconditions met), the ladder walks
// salvage → live_fix → redispatch; with all spent it yields "" (abort as
// today). Every decision carries a justification.
func TestNextCorrection_OrderIsLoadBearing(t *testing.T) {
	t.Parallel()
	in := interaction.CorrectionInput{
		Phase: "ship", Violation: "misplaced report",
		NamedREPL: true, Busy: false,
		DecisionID: "d1", RungBudget: fullBudget(2),
	}

	act := interaction.NextCorrection(in)
	if act.Rung != interaction.RungSalvage {
		t.Fatalf("rung 1 must be salvage (cheapest, no agent); got %q", act.Rung)
	}
	if act.Reason == "" {
		t.Error("every ladder decision must be justified (ADR-0044 invariant)")
	}

	in.RungBudget[interaction.RungSalvage] = 0
	act = interaction.NextCorrection(in)
	if act.Rung != interaction.RungLiveFix {
		t.Fatalf("salvage spent + idle named REPL ⇒ live_fix; got %q", act.Rung)
	}

	in.RungBudget[interaction.RungLiveFix] = 0
	act = interaction.NextCorrection(in)
	if act.Rung != interaction.RungRedispatch {
		t.Fatalf("salvage+live_fix spent ⇒ redispatch; got %q", act.Rung)
	}

	in.RungBudget[interaction.RungRedispatch] = 0
	act = interaction.NextCorrection(in)
	if act.Rung != "" {
		t.Fatalf("all budgets spent ⇒ exhausted (\"\"); got %q", act.Rung)
	}
}

// TestRung2_RequiresNamedSession_ElseSkipsToRedispatch — the H1 lifecycle
// constraint: live_fix needs a NAMED session preserved through the review
// gate AND an idle pane. Unnamed ⇒ rung 3; busy ⇒ rung 3 (never touch a
// working agent).
func TestRung2_RequiresNamedSession_ElseSkipsToRedispatch(t *testing.T) {
	t.Parallel()
	base := interaction.CorrectionInput{
		Phase: "build", Violation: "v", DecisionID: "d1",
	}

	t.Run("unnamed_skips_to_redispatch", func(t *testing.T) {
		in := base
		in.RungBudget = fullBudget(2)
		in.RungBudget[interaction.RungSalvage] = 0
		in.NamedREPL, in.Busy = false, false
		if act := interaction.NextCorrection(in); act.Rung != interaction.RungRedispatch {
			t.Errorf("no preserved named session ⇒ redispatch; got %q", act.Rung)
		}
	})

	t.Run("busy_pane_never_interrupted", func(t *testing.T) {
		in := base
		in.RungBudget = fullBudget(2)
		in.RungBudget[interaction.RungSalvage] = 0
		in.NamedREPL, in.Busy = true, true
		if act := interaction.NextCorrection(in); act.Rung != interaction.RungRedispatch {
			t.Errorf("busy pane ⇒ redispatch (never touch a working agent); got %q", act.Rung)
		}
	})
}

// TestNextCorrection_BudgetsExhaust — zero/negative/missing budgets are all
// "spent"; a nil budget map decides nothing (the caller aborts as today).
func TestNextCorrection_BudgetsExhaust(t *testing.T) {
	t.Parallel()
	in := interaction.CorrectionInput{Phase: "build", Violation: "v"}
	if act := interaction.NextCorrection(in); act.Rung != "" {
		t.Errorf("nil budget ⇒ exhausted; got %q", act.Rung)
	}
	in.RungBudget = map[string]int{
		interaction.RungSalvage:    0,
		interaction.RungLiveFix:    -1,
		interaction.RungRedispatch: 0,
	}
	if act := interaction.NextCorrection(in); act.Rung != "" {
		t.Errorf("spent budgets ⇒ exhausted; got %q", act.Rung)
	}
}
