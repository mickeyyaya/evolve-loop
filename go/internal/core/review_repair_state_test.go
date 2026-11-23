package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codereview"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const noFindingReport = "## Scope\nlightweight.\n## Findings\n## Verdict\nPASS\n"

func roundEvents(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == codereview.CodeRound {
			out = append(out, e)
		}
	}
	return out
}

func reloaded(t *testing.T, cr *cycleRun) CycleState {
	t.Helper()
	raw, err := json.Marshal(cr.o.storage.(*fakeStorage).cycleState)
	if err != nil {
		t.Fatal(err)
	}
	var cs CycleState
	if err := json.Unmarshal(raw, &cs); err != nil {
		t.Fatal(err)
	}
	return cs
}

func fullGap() float64 {
	return float64(len(qualityindex.Keys()) * qualityindex.DefaultThreshold)
}

func writeLedger(t *testing.T, cr *cycleRun, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(cr.cs.WorkspacePath, defectledger.LedgerFile), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReviewRepair_SnapshotsSurviveTwoPhaseBoundariesAndAReload(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	first := reloaded(t, cr)
	if first.ReviewRepair == nil || len(first.ReviewRepair.Rounds) != 1 || first.ReviewRepair.Rounds[0].GapMass != fullGap() {
		t.Fatalf("persisted after the review = %+v, want round 1 with the exact gap mass %v (the report has no scores)", first.ReviewRepair, fullGap())
	}
	if len(first.ReviewRepair.Rounds[0].Statuses) != 1 {
		t.Fatalf("statuses = %v, want the one finding", first.ReviewRepair.Rounds[0].Statuses)
	}
	for _, status := range first.ReviewRepair.Rounds[0].Statuses {
		if status != defectledger.StatusOpen {
			t.Fatalf("status = %q, want the shadow row read as OPEN", status)
		}
	}
	complete(t, cr, PhaseBuild, VerdictPASS)
	second := reloaded(t, cr)
	if !reflect.DeepEqual(second.ReviewRepair, first.ReviewRepair) {
		t.Fatalf("after the next phase = %+v, want the history carried unchanged: %+v", second.ReviewRepair, first.ReviewRepair)
	}
	cr.cs = second
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	third := reloaded(t, cr)
	if third.ReviewRepair == nil || len(third.ReviewRepair.Rounds) != 2 || !reflect.DeepEqual(third.ReviewRepair.Rounds[0], first.ReviewRepair.Rounds[0]) {
		t.Fatalf("persisted after round 2 = %+v, want round 1 kept and round 2 appended", third.ReviewRepair)
	}
	if rounds := roundEvents(*events); len(rounds) != 2 || rounds[1].Fields["round"] != "2" {
		t.Fatalf("REVIEW_ROUND = %+v, want one per review round", rounds)
	}
}

func TestReviewRepair_AStatusChangedBetweenRoundsReachesTheDecision(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	doc, _, err := defectledger.Read(cr.cs.WorkspacePath)
	if err != nil || len(doc.Entries) != 1 {
		t.Fatalf("ledger = %+v (err %v)", doc, err)
	}
	doc.Entries[0].Status, doc.Entries[0].Reason = defectledger.StatusFixed, ""
	if err := defectledger.Write(cr.cs.WorkspacePath, doc); err != nil {
		t.Fatal(err)
	}
	cr.cs = reloaded(t, cr)
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	second := reloaded(t, cr)
	for _, status := range second.ReviewRepair.Rounds[1].Statuses {
		if status != defectledger.StatusFixed {
			t.Fatalf("round 2 status = %q, want FIXED", status)
		}
	}
	for _, status := range second.ReviewRepair.Rounds[0].Statuses {
		if status != defectledger.StatusOpen {
			t.Fatalf("round 1 status = %q, want the earlier OPEN kept", status)
		}
	}
	rounds := roundEvents(*events)
	if got := rounds[1].Fields; got["decision"] != codereview.DecisionGapOpen || got["finding_mass"] != "0" {
		t.Fatalf("round 2 fields = %+v, want gap_open with no open finding mass", got)
	}
}

func TestReviewRepair_EmitsOneShadowDecisionWithItsMasses(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	rounds := roundEvents(*events)
	if len(rounds) != 1 {
		t.Fatalf("REVIEW_ROUND events = %d, want 1", len(rounds))
	}
	f := rounds[0].Fields
	if f["stage"] != "shadow" || f["round"] != "1" || f["finding_mass"] != "4" || f["gap_mass"] != formatFloat(fullGap()) || f["decision"] == "" {
		t.Fatalf("fields = %+v, want shadow round 1, the HIGH row mass 4 and gap %v", f, fullGap())
	}
}

func formatFloat(v float64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestReviewRepair_AReadFaultOfTheLedgerAfterAGoodRecordIsALedgerDecisionWithNoHistory(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	if err := os.WriteFile(filepath.Join(cr.cs.WorkspacePath, phasecontract.ArtifactFilename(codereview.PhaseName)), []byte(noFindingReport), 0o644); err != nil {
		t.Fatal(err)
	}
	writeLedger(t, cr, "{not json")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	rounds := roundEvents(*events)
	if len(rounds) != 1 || rounds[0].Fields["decision"] != codereview.DecisionLedger {
		t.Fatalf("REVIEW_ROUND = %+v, want one ledger decision", rounds)
	}
	if got := reloaded(t, cr).ReviewRepair; got != nil {
		t.Fatalf("history = %+v, want none when the ledger could not be read", got)
	}
}

func TestReviewRepair_AFailedRecordIsALedgerDecisionWithNoHistory(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	writeLedger(t, cr, "{not json")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	rounds := roundEvents(*events)
	if len(rounds) != 1 || rounds[0].Fields["decision"] != codereview.DecisionLedger {
		t.Fatalf("REVIEW_ROUND = %+v, want one ledger decision", rounds)
	}
	if got := reloaded(t, cr).ReviewRepair; got != nil {
		t.Fatalf("history = %+v, want none: a false zero gap must not enter the history", got)
	}
}

func TestReviewRepair_AGoodRoundAfterAFaultedRoundIsHistoryNotACleanBaseline(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	writeLedger(t, cr, "{not json")
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	if err := os.Remove(filepath.Join(cr.cs.WorkspacePath, defectledger.LedgerFile)); err != nil {
		t.Fatal(err)
	}
	cr.cs = reloaded(t, cr)
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	rounds := roundEvents(*events)
	if len(rounds) != 2 || rounds[1].Fields["decision"] != codereview.DecisionHistory {
		t.Fatalf("REVIEW_ROUND = %+v, want round 2 to report history: round 1 was never observed", rounds)
	}
	got := reloaded(t, cr).ReviewRepair
	if got == nil || len(got.Rounds) != 2 || got.Rounds[0].Observed || !got.Rounds[1].Observed {
		t.Fatalf("history = %+v, want round 1 unobserved and round 2 observed", got)
	}
}

func TestReviewRepair_AMissingReportIsALedgerDecisionWithNoHistoryEvenWhenTheLedgerReads(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build")
	if err := os.Remove(filepath.Join(cr.cs.WorkspacePath, phasecontract.ArtifactFilename(codereview.PhaseName))); err != nil {
		t.Fatal(err)
	}
	complete(t, cr, Phase(codereview.PhaseName), VerdictPASS)
	rounds := roundEvents(*events)
	if len(rounds) != 1 || rounds[0].Fields["decision"] != codereview.DecisionLedger {
		t.Fatalf("REVIEW_ROUND = %+v, want one ledger decision", rounds)
	}
	if got := reloaded(t, cr).ReviewRepair; got != nil {
		t.Fatalf("history = %+v, want none: the failed Record must not be overwritten by a clean ledger read", got)
	}
}

func TestReviewRepair_FailedSkippedAndOtherPhasesLeaveTheHistoryAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		phase   Phase
		verdict string
	}{{Phase(codereview.PhaseName), VerdictFAIL}, {Phase(codereview.PhaseName), VerdictSKIPPED}, {PhaseBuild, VerdictPASS}} {
		cr, events := reviewHarness(t, "scout", "build")
		complete(t, cr, tc.phase, tc.verdict)
		if cr.cs.ReviewRepair != nil || len(roundEvents(*events)) != 0 {
			t.Errorf("%s/%s: state %+v, round events %d, want none", tc.phase, tc.verdict, cr.cs.ReviewRepair, len(roundEvents(*events)))
		}
	}
}

func TestReviewRepair_BatchPathRecordsTheSameHistory(t *testing.T) {
	t.Parallel()

	cr, events := reviewHarness(t, "scout", "build", codereview.PhaseName)
	cr.recordBatchMember(Phase(codereview.PhaseName), PhaseResponse{Verdict: VerdictPASS})
	if cr.cs.ReviewRepair == nil || len(cr.cs.ReviewRepair.Rounds) != 1 || cr.cs.ReviewRepair.Rounds[0].GapMass != fullGap() {
		t.Fatalf("batch path state = %+v, want round 1 recorded on cr.cs with gap %v", cr.cs.ReviewRepair, fullGap())
	}
	if len(roundEvents(*events)) != 1 || len(reviewEvents(*events)) != 1 {
		t.Fatalf("events: %d round, %d findings, want one of each", len(roundEvents(*events)), len(reviewEvents(*events)))
	}
}

func TestReviewRepair_ShadowDecisionChangesNoRouting(t *testing.T) {
	t.Parallel()

	review, _ := reviewHarness(t, "scout", "build")
	control, _ := reviewHarness(t, "scout", "build")
	complete(t, review, Phase(codereview.PhaseName), VerdictPASS)
	complete(t, control, PhaseBuild, VerdictPASS)
	if review.scheduledNext != "" || review.scheduledNext != control.scheduledNext {
		t.Fatalf("scheduledNext = %q (control %q), want the live schedule untouched", review.scheduledNext, control.scheduledNext)
	}
	if review.current != Phase(codereview.PhaseName) {
		t.Fatalf("current = %q, want the review phase recorded as before", review.current)
	}
	doc, _, err := defectledger.Read(review.cs.WorkspacePath)
	if err != nil || len(doc.Entries) != 1 || doc.Entries[0].Status != defectledger.StatusDeferred || doc.Entries[0].Reason != codereview.ShadowReason {
		t.Fatalf("ledger = %+v (err %v), want the one row still DEFERRED with the shadow reason", doc, err)
	}
	review.cs.ReviewRepair = nil
	review.cs.CompletedPhases, control.cs.CompletedPhases = nil, nil
	review.cs.Phase, control.cs.Phase = "", ""
	review.cs.WorkspacePath, control.cs.WorkspacePath = "", ""
	review.cs.FinalVerdict, control.cs.FinalVerdict = "", ""
	if !reflect.DeepEqual(review.cs, control.cs) {
		t.Fatalf("cycle state differs beyond review_repair:\n%+v\n%+v", review.cs, control.cs)
	}
}
