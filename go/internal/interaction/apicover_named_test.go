package interaction_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func recordAndRollup(t *testing.T, recs []interaction.Outcome) interaction.Summary {
	t.Helper()
	ws := t.TempDir()
	rec := interaction.NewRecorder(ws)
	for _, o := range recs {
		rec.Record(o)
	}
	s, ok := interaction.Rollup(ws)
	if !ok {
		t.Fatalf("Rollup must report ok=true after %d recorded outcomes", len(recs))
	}
	return s
}

func TestCorrectionLadderResultConsts_FlowThroughRollup(t *testing.T) {
	t.Parallel()
	salv := func(result string) interaction.Outcome {
		return interaction.Outcome{
			Event: interaction.Event{
				Kind: interaction.KindSalvage, Phase: "ship", Cycle: 1,
				Trigger: "contract_reject", Rung: interaction.RungSalvage, DecisionID: "d1",
			},
			Result: result,
		}
	}
	redis := func(result string) interaction.Outcome {
		return interaction.Outcome{
			Event: interaction.Event{
				Kind: interaction.KindCorrectionRedispatch, Phase: "ship", Cycle: 1,
				Trigger: "contract_reject", Rung: interaction.RungRedispatch, DecisionID: "d1",
			},
			Result: result,
		}
	}
	s := recordAndRollup(t, []interaction.Outcome{
		salv(interaction.ResultWouldAct),
		salv(interaction.ResultFoundButInvalid),
		salv(interaction.ResultNotFound),
		redis(interaction.ResultDispatchFailed),
		redis(interaction.ResultNonCanonicalVerdict),
		redis(interaction.ResultRejectedAgain),
		redis(interaction.ResultQuotaDeferred),
	})
	if s.ByResult[interaction.ResultWouldAct] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultWouldAct, s.ByResult[interaction.ResultWouldAct])
	}
	if s.ByResult[interaction.ResultFoundButInvalid] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultFoundButInvalid, s.ByResult[interaction.ResultFoundButInvalid])
	}
	if s.ByResult[interaction.ResultNotFound] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultNotFound, s.ByResult[interaction.ResultNotFound])
	}
	if s.ByResult[interaction.ResultDispatchFailed] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultDispatchFailed, s.ByResult[interaction.ResultDispatchFailed])
	}
	if s.ByResult[interaction.ResultQuotaDeferred] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultQuotaDeferred, s.ByResult[interaction.ResultQuotaDeferred])
	}
	if s.ByResult[interaction.ResultNonCanonicalVerdict] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultNonCanonicalVerdict, s.ByResult[interaction.ResultNonCanonicalVerdict])
	}
	if s.ByResult[interaction.ResultRejectedAgain] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultRejectedAgain, s.ByResult[interaction.ResultRejectedAgain])
	}
	if s.ByRung[interaction.RungSalvage] != 3 || s.ByRung[interaction.RungRedispatch] != 4 {
		t.Errorf("rung distribution wrong: %v", s.ByRung)
	}
	if s.Decisions != 1 {
		t.Errorf("Decisions = %d, want 1 (all rungs share DecisionID d1)", s.Decisions)
	}
}

func TestAutoRespondResultConsts_FlowThroughRollup(t *testing.T) {
	t.Parallel()
	ev := func(result string) interaction.Outcome {
		return interaction.Outcome{
			Event:  interaction.Event{Kind: interaction.KindAutoRespond, Phase: "build", Cycle: 2, Trigger: "unknown_prompt"},
			Result: result,
		}
	}
	s := recordAndRollup(t, []interaction.Outcome{
		ev(interaction.ResultPromptCleared),
		ev(interaction.ResultSuppressedLingering),
		ev(interaction.ResultRunEnded),
	})
	if s.ByResult[interaction.ResultPromptCleared] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultPromptCleared, s.ByResult[interaction.ResultPromptCleared])
	}
	if s.ByResult[interaction.ResultSuppressedLingering] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultSuppressedLingering, s.ByResult[interaction.ResultSuppressedLingering])
	}
	if s.ByResult[interaction.ResultRunEnded] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultRunEnded, s.ByResult[interaction.ResultRunEnded])
	}
	if s.ByRung["none"] != 3 {
		t.Errorf("ByRung[none] = %d, want 3 (auto-respond carries no ladder rung)", s.ByRung["none"])
	}
}

func TestRollup_EmptyWorkspaceReportsNotOk(t *testing.T) {
	t.Parallel()
	if s, ok := interaction.Rollup(""); ok {
		t.Errorf("Rollup(\"\") must report ok=false; got %+v", s)
	}
	if s, ok := interaction.Rollup(t.TempDir()); ok {
		t.Errorf("Rollup over an empty dir must report ok=false; got %+v", s)
	}
}

func TestCorrectionAction_BoundViaNextCorrection(t *testing.T) {
	t.Parallel()
	var act interaction.CorrectionAction = interaction.NextCorrection(interaction.CorrectionInput{
		Phase:     "ship",
		Violation: "misplaced report",
		NamedREPL: true,
		Busy:      false,
		RungBudget: map[string]int{
			interaction.RungSalvage:    1,
			interaction.RungLiveFix:    1,
			interaction.RungRedispatch: 2,
		},
	})
	if act.Rung != interaction.RungSalvage {
		t.Errorf("CorrectionAction.Rung = %q, want %q (cheapest rung)", act.Rung, interaction.RungSalvage)
	}
	if act.Reason == "" {
		t.Error("CorrectionAction.Reason must justify the choice (ADR-0044 invariant)")
	}
	exhausted := interaction.NextCorrection(interaction.CorrectionInput{Phase: "ship"})
	if exhausted.Rung != "" {
		t.Errorf("exhausted budget ⇒ CorrectionAction.Rung == \"\"; got %q", exhausted.Rung)
	}
	if exhausted.Reason == "" {
		t.Error("the exhausted decision must still carry a Reason")
	}
}

func TestInteractionRule_BoundViaLoadRules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	corpus := []string{"healthy banner line"}
	const regex = "Rate this session before exiting"
	id, err := interaction.PromoteRule(dir, regex, "1,Enter", "agy session rating", corpus)
	if err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	rules := interaction.LoadRules(dir, corpus)
	if len(rules) != 1 {
		t.Fatalf("LoadRules returned %d rules, want 1", len(rules))
	}
	var r interaction.InteractionRule = rules[0]
	if r.ID != id {
		t.Errorf("InteractionRule.ID = %q, want %q", r.ID, id)
	}
	if r.Regex != regex {
		t.Errorf("InteractionRule.Regex = %q, want %q", r.Regex, regex)
	}
	if r.ResponseKeys != "1,Enter" {
		t.Errorf("InteractionRule.ResponseKeys = %q, want %q", r.ResponseKeys, "1,Enter")
	}
	if r.Stage != interaction.RuleStageShadow {
		t.Errorf("InteractionRule.Stage = %q, want %q (promotion lands shadow)", r.Stage, interaction.RuleStageShadow)
	}
	if _, statErr := os.Stat(filepath.Join(dir, id+".yaml")); statErr != nil {
		t.Errorf("PromoteRule must have written %s.yaml: %v", id, statErr)
	}
}

func TestSubmitVerifyConsts_FlowThroughRollup(t *testing.T) {
	ev := func(result string) interaction.Outcome {
		return interaction.Outcome{
			Event:  interaction.Event{Kind: interaction.KindSubmitVerify, Phase: "build", Cycle: 1526, Trigger: "driver_submission"},
			Result: result,
		}
	}
	s := recordAndRollup(t, []interaction.Outcome{
		ev(interaction.ResultSubmitVerified),
		ev(interaction.ResultSubmittedAfterResend),
		ev(interaction.ResultSubmitWedged),
		ev(interaction.ResultNotVerified),
	})
	if got := s.ByKind[interaction.KindSubmitVerify]; got != 4 {
		t.Errorf("ByKind[%q] = %d, want 4", interaction.KindSubmitVerify, got)
	}
	for _, r := range []string{
		interaction.ResultSubmitVerified,
		interaction.ResultSubmittedAfterResend,
		interaction.ResultSubmitWedged,
		interaction.ResultNotVerified,
	} {
		if got := s.ByResult[r]; got != 1 {
			t.Errorf("ByResult[%q] = %d, want 1 — the const must be the exact key that flows through", r, got)
		}
	}
	if got := s.ByResult[interaction.ResultPromptCleared]; got != 0 {
		t.Errorf("ByResult[%q] = %d, want 0 — submit-verify no-ops must not dilute injection-success", interaction.ResultPromptCleared, got)
	}
}
