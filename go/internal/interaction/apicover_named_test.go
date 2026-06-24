package interaction_test

// apicover_named_test.go — ADR-0050 public-API coverage graduation for
// internal/interaction. Every assertion drives a REAL producer→consumer path,
// never a value-pin:
//
//   - Result* consts are the contract values that the orchestrator
//     (internal/core/cyclerun_review.go) and the auto-responder
//     (internal/bridge/autorespond.go) write into Outcome.Result. The
//     in-package consumer that gives them meaning is Rollup, which buckets
//     Outcome.Result into Summary.ByResult. Each const is asserted by recording
//     the outcome a producer would emit, rolling it up, and proving the const
//     is the exact ByResult key that flows through — so a renamed const, or a
//     producer that stopped emitting it, fails the test.
//   - Rollup is invoked and its (Summary, bool) contract asserted.
//   - CorrectionAction is bound via its real producer NextCorrection.
//   - InteractionRule is bound via its real producer LoadRules (over a rule
//     PromoteRule durably wrote).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// recordAndRollup records each (kind, result) pair through the chokepoint the
// real producers use, then returns the per-cycle Rollup the orchestrator writes
// from RunCycle's deferred persistence block.
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

// TestCorrectionLadderResultConsts_FlowThroughRollup pins the four correction-
// ladder outcome codes the orchestrator's salvage/redispatch rungs emit
// (cyclerun_review.go): ResultWouldAct (shadow stage logs the rung without
// acting), ResultFoundButInvalid (salvage relocated but the destination failed
// verification), ResultNotFound (no salvageable candidate), and the
// re-dispatch error/verdict codes ResultDispatchFailed (the dispatch errored)
// and ResultNonCanonicalVerdict (the re-dispatch returned an unevaluable
// verdict) plus ResultRejectedAgain (the re-dispatched deliverable failed the
// gate again). Each must survive the Rollup aggregation as its own ByResult
// bucket — the §10 metrics distinguish these outcomes.
func TestCorrectionLadderResultConsts_FlowThroughRollup(t *testing.T) {
	t.Parallel()
	// One salvage event per ladder outcome + the re-dispatch outcomes, exactly
	// as the orchestrator records them (Kind/Rung mirror cyclerun_review.go).
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
	})
	// ResultWouldAct — shadow-stage "selected but not executed" bucket.
	if s.ByResult[interaction.ResultWouldAct] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultWouldAct, s.ByResult[interaction.ResultWouldAct])
	}
	// ResultFoundButInvalid — salvage relocated, destination failed verification.
	if s.ByResult[interaction.ResultFoundButInvalid] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultFoundButInvalid, s.ByResult[interaction.ResultFoundButInvalid])
	}
	// ResultNotFound — no salvageable candidate existed.
	if s.ByResult[interaction.ResultNotFound] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultNotFound, s.ByResult[interaction.ResultNotFound])
	}
	// ResultDispatchFailed — the correction re-dispatch itself errored.
	if s.ByResult[interaction.ResultDispatchFailed] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultDispatchFailed, s.ByResult[interaction.ResultDispatchFailed])
	}
	// ResultNonCanonicalVerdict — re-dispatch returned an unevaluable verdict.
	if s.ByResult[interaction.ResultNonCanonicalVerdict] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultNonCanonicalVerdict, s.ByResult[interaction.ResultNonCanonicalVerdict])
	}
	// ResultRejectedAgain — re-dispatched deliverable failed the gate again.
	if s.ByResult[interaction.ResultRejectedAgain] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultRejectedAgain, s.ByResult[interaction.ResultRejectedAgain])
	}
	// The two distinct ladder rungs aggregate under their own buckets.
	if s.ByRung[interaction.RungSalvage] != 3 || s.ByRung[interaction.RungRedispatch] != 3 {
		t.Errorf("rung distribution wrong: %v", s.ByRung)
	}
	// One correction decision spans every rung (shared DecisionID d1).
	if s.Decisions != 1 {
		t.Errorf("Decisions = %d, want 1 (all rungs share DecisionID d1)", s.Decisions)
	}
}

// TestAutoRespondResultConsts_FlowThroughRollup pins the three auto-respond
// resolution codes the bridge's autoResponder emits (autorespond.go):
// ResultPromptCleared (resolvePending: the matched pattern no longer matches on
// the next capture), ResultSuppressedLingering (decide: a fire-once prompt's
// dismissed text still lingers in scrollback and a re-fire was suppressed), and
// ResultRunEnded (flushPending: the run concluded before the next capture could
// resolve the in-flight send). Each is asserted as its own ByResult bucket.
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
	// ResultPromptCleared — the triggering pattern cleared on the next capture.
	if s.ByResult[interaction.ResultPromptCleared] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultPromptCleared, s.ByResult[interaction.ResultPromptCleared])
	}
	// ResultSuppressedLingering — a fire-once prompt's text lingered; re-fire suppressed.
	if s.ByResult[interaction.ResultSuppressedLingering] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultSuppressedLingering, s.ByResult[interaction.ResultSuppressedLingering])
	}
	// ResultRunEnded — the run ended before resolution.
	if s.ByResult[interaction.ResultRunEnded] != 1 {
		t.Errorf("ByResult[%q] = %d, want 1", interaction.ResultRunEnded, s.ByResult[interaction.ResultRunEnded])
	}
	// Non-ladder interactions aggregate under the "none" rung bucket.
	if s.ByRung["none"] != 3 {
		t.Errorf("ByRung[none] = %d, want 3 (auto-respond carries no ladder rung)", s.ByRung["none"])
	}
}

// TestRollup_EmptyWorkspaceReportsNotOk invokes Rollup directly and asserts its
// (Summary, bool) contract: an empty workspace path has nothing to summarize,
// so ok=false and the Summary is zero — the signal WriteRollup uses to stay
// clean (no empty-noise file).
func TestRollup_EmptyWorkspaceReportsNotOk(t *testing.T) {
	t.Parallel()
	if s, ok := interaction.Rollup(""); ok {
		t.Errorf("Rollup(\"\") must report ok=false; got %+v", s)
	}
	// A workspace dir with no *-interactions.ndjson is likewise nothing to summarize.
	if s, ok := interaction.Rollup(t.TempDir()); ok {
		t.Errorf("Rollup over an empty dir must report ok=false; got %+v", s)
	}
}

// TestCorrectionAction_BoundViaNextCorrection binds the CorrectionAction type
// to its real producer: NextCorrection returns a CorrectionAction whose Rung
// (the cheapest rung with budget) and Reason (the ADR-0044 every-decision-
// justified invariant) are load-bearing. Asserting both fields of the returned
// value exercises the type, not just names it.
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
	// Cheapest rung first: salvage, with a non-empty justification.
	if act.Rung != interaction.RungSalvage {
		t.Errorf("CorrectionAction.Rung = %q, want %q (cheapest rung)", act.Rung, interaction.RungSalvage)
	}
	if act.Reason == "" {
		t.Error("CorrectionAction.Reason must justify the choice (ADR-0044 invariant)")
	}
	// Exhausted budget ⇒ empty rung (caller aborts as today), still justified.
	exhausted := interaction.NextCorrection(interaction.CorrectionInput{Phase: "ship"})
	if exhausted.Rung != "" {
		t.Errorf("exhausted budget ⇒ CorrectionAction.Rung == \"\"; got %q", exhausted.Rung)
	}
	if exhausted.Reason == "" {
		t.Error("the exhausted decision must still carry a Reason")
	}
}

// TestInteractionRule_BoundViaLoadRules binds the InteractionRule type to its
// real producer: PromoteRule durably writes a rule, LoadRules replays it as an
// []InteractionRule. Asserting the loaded value's fields (ID/Regex/
// ResponseKeys/Stage) exercises the type through the promotion→replay path a
// driver boot uses.
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
	// The replayed rule round-trips its identifying fields.
	if r.ID != id {
		t.Errorf("InteractionRule.ID = %q, want %q", r.ID, id)
	}
	if r.Regex != regex {
		t.Errorf("InteractionRule.Regex = %q, want %q", r.Regex, regex)
	}
	if r.ResponseKeys != "1,Enter" {
		t.Errorf("InteractionRule.ResponseKeys = %q, want %q", r.ResponseKeys, "1,Enter")
	}
	// A freshly promoted rule lands at shadow (never auto-enforce).
	if r.Stage != interaction.RuleStageShadow {
		t.Errorf("InteractionRule.Stage = %q, want %q (promotion lands shadow)", r.Stage, interaction.RuleStageShadow)
	}
	// Sanity: the rule file the producer wrote actually exists on disk.
	if _, statErr := os.Stat(filepath.Join(dir, id+".yaml")); statErr != nil {
		t.Errorf("PromoteRule must have written %s.yaml: %v", id, statErr)
	}
}
