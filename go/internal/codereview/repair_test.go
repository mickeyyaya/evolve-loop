package codereview

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

func defaultConfig() policy.ConvergenceConfig { return policy.Policy{}.ConvergenceConfig() }

func row(id, severity string) defectledger.Entry {
	return defectledger.Entry{ID: id, Severity: severity, Source: PhaseName, Status: defectledger.StatusDeferred, Reason: ShadowReason}
}

func snap(gap float64, statuses ...string) cyclestate.RoundSnapshot {
	s := cyclestate.RoundSnapshot{Observed: true, GapMass: gap, Statuses: map[string]string{}}
	for i := 0; i+1 < len(statuses); i += 2 {
		s.Statuses[statuses[i]] = statuses[i+1]
	}
	return s
}

func decide(round int, rows []defectledger.Entry, snaps ...cyclestate.RoundSnapshot) RoundDecision {
	return DecideAfterReview(RoundInput{Round: round, Rows: rows, Snapshots: snaps, Config: defaultConfig()})
}

func TestGapMass_SumsThresholdMinusScoreOverGaps(t *testing.T) {
	t.Parallel()

	thresholds := qualityindex.Thresholds{}
	scores := qualityindex.Scores{}
	for _, key := range qualityindex.Keys() {
		thresholds[key] = 4
		scores[key] = qualityindex.Score{Value: 4}
	}
	if got := GapMass(scores, thresholds); got != 0 {
		t.Fatalf("qualifying scores gap mass = %v, want 0", got)
	}
	scores[qualityindex.Keys()[0]] = qualityindex.Score{Value: 1}
	if got := GapMass(scores, thresholds); got != 3 {
		t.Fatalf("one dimension at 1 against 4: gap mass = %v, want 3", got)
	}
	if got := GapMass(qualityindex.Scores{}, thresholds); got != float64(4*len(qualityindex.Keys())) {
		t.Fatalf("no scores: gap mass = %v, want every dimension at its full threshold", got)
	}
}

func TestTakeSnapshot_ReadsShadowRowsAsOpenKeepsOtherStatusesAndSkipsOtherSources(t *testing.T) {
	t.Parallel()

	fixed := row("d2", "HIGH")
	fixed.Status, fixed.Reason = defectledger.StatusFixed, ""
	foreign := row("d3", "HIGH")
	foreign.Source = "audit"
	thresholds := qualityindex.Thresholds{}
	got := TakeSnapshot(qualityindex.Scores{}, thresholds, []defectledger.Entry{row("d1", "HIGH"), fixed, foreign})
	want := map[string]string{"d1": "OPEN", "d2": "FIXED"}
	if len(got.Statuses) != 2 || got.Statuses["d1"] != want["d1"] || got.Statuses["d2"] != want["d2"] {
		t.Fatalf("statuses = %v, want %v", got.Statuses, want)
	}
	if !got.Observed {
		t.Fatal("a taken snapshot must be marked observed")
	}
	if got.GapMass != 0 {
		t.Fatalf("gap mass = %v, want 0 with no thresholds", got.GapMass)
	}
}

func TestDecideAfterReview_LedgerFaultNeverCallsDecide(t *testing.T) {
	t.Parallel()

	got := DecideAfterReview(RoundInput{Round: 1, Snapshots: []cyclestate.RoundSnapshot{snap(0, "d1", "OPEN")}, LedgerErr: errors.New("disk"), Rows: []defectledger.Entry{row("d1", "HIGH")}, Config: defaultConfig()})
	if got.Decision != DecisionLedger || got.Reasons != nil || got.Rung != 0 {
		t.Fatalf("decision = %+v, want the ledger stop with no convergence output", got)
	}
}

func TestDecideAfterReview_HistoryOfTheWrongLengthIsItsOwnDecision(t *testing.T) {
	t.Parallel()

	for _, in := range []RoundInput{
		{Round: 2, Snapshots: []cyclestate.RoundSnapshot{snap(0)}},
		{Round: 1, Snapshots: []cyclestate.RoundSnapshot{snap(0), snap(0)}},
		{Round: 0},
	} {
		in.Config = defaultConfig()
		if got := DecideAfterReview(in); got.Decision != DecisionHistory {
			t.Errorf("round %d with %d snapshots: decision %q, want %q", in.Round, len(in.Snapshots), got.Decision, DecisionHistory)
		}
	}
}

func TestDecideAfterReview_ResolvedNeedsNoOpenRowAndNoGap(t *testing.T) {
	t.Parallel()

	got := decide(1, []defectledger.Entry{row("d1", "HIGH")}, snap(0, "d1", "FIXED"))
	if got.Decision != DecisionResolved || got.FindingMass != 0 || got.GapMass != 0 {
		t.Fatalf("decision = %+v, want resolved", got)
	}
}

func TestDecideAfterReview_AGapWithNoFindingIsNotALanding(t *testing.T) {
	t.Parallel()

	got := decide(1, nil, snap(3))
	if got.Decision != DecisionGapOpen || got.GapMass != 3 {
		t.Fatalf("decision = %+v, want gap_open", got)
	}
}

func TestDecideAfterReview_AnOpenGapBlocksALandingEvenWhenOnlyAFindingBelowTheBarRemains(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "LOW")}
	got := decide(2, rows, snap(0, "d1", "OPEN"), snap(9, "d1", "FIXED", "d2", "OPEN"))
	if got.Decision != DecisionGapOpen {
		t.Fatalf("decision = %q, want gap_open: convergence lands on the findings at the bar, but the gap of 9 is open", got.Decision)
	}
	closed := decide(2, rows, snap(0, "d1", "OPEN"), snap(0, "d1", "FIXED", "d2", "OPEN"))
	if closed.Decision != string(convergence.ActionLand) {
		t.Fatalf("decision = %q, want Land once the gap is closed", closed.Decision)
	}
}

func TestDecideAfterReview_OtherSourcesDoNotCount(t *testing.T) {
	t.Parallel()

	foreign := row("d1", "CRITICAL")
	foreign.Source = "audit"
	got := decide(1, []defectledger.Entry{foreign}, snap(0, "d1", "OPEN"))
	if got.Decision != DecisionResolved {
		t.Fatalf("decision = %+v, want a row of another source ignored", got)
	}
}

func TestDecideAfterReview_DelegatesToDecideOverTheSnapshots(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "LOW")}
	got := decide(1, rows, snap(2, "d1", "OPEN", "d2", "OPEN"))
	want := convergence.Decide(convergence.Input{
		Loop: convergence.LoopCodeReview, Round: 0, Config: defaultConfig(),
		Rounds: []convergence.Judgment{{Findings: []convergence.Finding{
			{ID: "d1", Severity: convergence.SeverityHigh, Status: convergence.StatusOpen},
			{ID: "d2", Severity: convergence.SeverityLow, Status: convergence.StatusOpen},
		}}},
		Mass: func(_ int, atBar []convergence.Finding) float64 { return convergence.FindingMass(atBar) + 2 },
	})
	if got.Decision != string(want.Action) || got.Rung != want.Rung || !slices.Equal(got.Reasons, want.Reasons) || got.BlockingBar != string(want.BlockingBar) {
		t.Fatalf("decision = %+v, want Decide's own: action %s rung %d reasons %v", got, want.Action, want.Rung, want.Reasons)
	}
	if got.FindingMass != 5 {
		t.Fatalf("finding mass = %v, want HIGH 4 + LOW 1", got.FindingMass)
	}
}

func TestDecideAfterReview_DecideMassIsTheMassConvergenceUsesAtItsBlockingBar(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "MEDIUM")}
	open := snap(0, "d1", "OPEN", "d2", "OPEN")
	got := decide(2, rows, open, snap(1, "d1", "OPEN", "d2", "OPEN"))
	if got.Rung != 2 || got.BlockingBar != defaultConfig().RaisedBlockingBar {
		t.Fatalf("decision = %+v, want rung 2 at the raised bar (an unchanged round spends the gain)", got)
	}
	if got.DecideMass != 4+1 {
		t.Fatalf("decide mass = %v, want the HIGH row 4 plus gap 1, without the MEDIUM row below the raised bar", got.DecideMass)
	}
	if got.FindingMass != 6 {
		t.Fatalf("finding mass = %v, want all OPEN rows 4+2", got.FindingMass)
	}
}

func TestDecideAfterReview_RoundOneDoesNotSeeARowRaisedInRoundTwo(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "HIGH")}
	one := decide(1, rows, snap(0, "d1", "OPEN"))
	if one.FindingMass != 4 {
		t.Fatalf("round 1 finding mass = %v, want 4: d2 is not in the round-1 snapshot", one.FindingMass)
	}
	two := decide(2, rows, snap(0, "d1", "OPEN"), snap(0, "d1", "OPEN", "d2", "OPEN"))
	if two.FindingMass != 8 {
		t.Fatalf("round 2 finding mass = %v, want 8", two.FindingMass)
	}
}

func TestDecideAfterReview_ARowFixedBetweenRoundsIsARepairAndLowersTheRung(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "HIGH")}
	before := snap(0, "d1", "OPEN", "d2", "OPEN")
	repaired := decide(2, rows, before, snap(0, "d1", "FIXED", "d2", "OPEN"))
	stuck := decide(2, rows, before, snap(0, "d1", "OPEN", "d2", "OPEN"))
	if repaired.Rung != 1 || stuck.Rung != 2 {
		t.Fatalf("rungs: repaired %d, unchanged %d, want 1 and 2", repaired.Rung, stuck.Rung)
	}
	if hasReason(repaired.Reasons, "CONVERGENCE_REPAIR_DAMAGE") || !hasReason(stuck.Reasons, "damage=0 repairs=0") {
		t.Fatalf("reasons: repaired %v, unchanged %v, want the repair to keep the gain unspent", repaired.Reasons, stuck.Reasons)
	}
}

func TestDecideAfterReview_AFixedRowThatOpensAgainIsDamage(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH")}
	got := decide(3, rows, snap(0, "d1", "OPEN"), snap(0, "d1", "FIXED"), snap(0, "d1", "OPEN"))
	if !hasReason(got.Reasons, "damage=1 repairs=0") {
		t.Fatalf("reasons = %v, want the reopen counted as damage", got.Reasons)
	}
}

func TestDecideAfterReview_AGapThatGrewSpendsTheGainEvenAfterARepair(t *testing.T) {
	t.Parallel()

	rows := []defectledger.Entry{row("d1", "HIGH"), row("d2", "HIGH")}
	before := func(gap float64) cyclestate.RoundSnapshot { return snap(gap, "d1", "OPEN", "d2", "OPEN") }
	after := snap(6, "d1", "FIXED", "d2", "OPEN")
	grew := decide(2, rows, before(0), after)
	flat := decide(2, rows, before(6), after)
	if grew.Rung != 2 || flat.Rung != 1 {
		t.Fatalf("rungs: gap grew from 0 to 6 = %d, gap flat at 6 = %d, want 2 and 1: the past round's own gap is the baseline", grew.Rung, flat.Rung)
	}
}

func TestDecideAfterReview_AnUnobservedRoundIsHistoryNotACleanRound(t *testing.T) {
	t.Parallel()

	padded := (*cyclestate.ReviewRepair)(nil).WithRound(2, snap(0, "d1", "OPEN"))
	rows := []defectledger.Entry{row("d1", "HIGH")}
	got := DecideAfterReview(RoundInput{Round: 2, Rows: rows, Snapshots: padded.Snapshots(), Config: defaultConfig()})
	if got.Decision != DecisionHistory || got.Reasons != nil {
		t.Fatalf("decision = %+v, want history: round 1 was never observed", got)
	}
}

func TestDecideAfterReview_TheOrderOfFindingsDoesNotDependOnMapIteration(t *testing.T) {
	t.Parallel()

	statuses := map[string]string{}
	for _, id := range []string{"d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8"} {
		statuses[id] = "OPEN"
	}
	snapshot := cyclestate.RoundSnapshot{Observed: true, Statuses: statuses}
	first := findingsOf(map[string]convergence.Severity{}, snapshot)
	ids := make([]string, len(first))
	for i, f := range first {
		ids[i] = f.ID
	}
	if !slices.IsSorted(ids) {
		t.Fatalf("finding order = %v, want sorted by id", ids)
	}
	for i := 0; i < 20; i++ {
		again := findingsOf(map[string]convergence.Severity{}, snapshot)
		if !slices.Equal(first, again) {
			t.Fatalf("run %d order differs: %v vs %v", i, first, again)
		}
	}
}

func TestRoundEvent_CarriesTheDecisionFields(t *testing.T) {
	t.Parallel()

	e := RoundEvent(Request{Cycle: 7, Round: 2}, "shadow", RoundDecision{Decision: "Stop", FindingMass: 4, GapMass: 1.5, DecideMass: 5.5, Rung: 2, BlockingBar: "HIGH", Reasons: []string{"a", "b"}})
	want := map[string]string{"decision": "Stop", "stage": "shadow", "round": "2", "finding_mass": "4", "gap_mass": "1.5", "decide_mass": "5.5", "rung": "2", "blocking_bar": "HIGH", "reasons": "a; b"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("field %s = %q, want %q", k, e.Fields[k], v)
		}
	}
	if e.Cycle != 7 || e.Code != CodeRound {
		t.Errorf("event = %+v, want cycle 7 and code REVIEW_ROUND", e)
	}
}

func hasReason(reasons []string, part string) bool {
	return slices.ContainsFunc(reasons, func(r string) bool { return strings.Contains(r, part) })
}
