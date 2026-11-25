package stageevidence_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

func amplifiedHeld(id, seq int) stageevidence.Record {
	return stageevidence.Record{Key: "held_proposals.stage", Version: currentVersion, Kind: stageevidence.KindDecision, ID: id, Seq: seq, Fields: map[string]string{"cycle": "1"}}
}

func TestAmplifiedDependsOnConflictRejectsEmptyAndNearMissKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "held_proposals", "held_proposals.stage ", "HELD_PROPOSALS.STAGE", "x.held_proposals.stage"} {
		if stageevidence.DependsOnConflict(key) {
			t.Errorf("DependsOnConflict(%q) = true, want false", key)
		}
	}
}

func TestAmplifiedOneUnmeasuredOutcomeAmongMeasuredBlocksReadiness(t *testing.T) {
	t.Parallel()
	chain := stageevidence.Chain{
		Records: linked([]stageevidence.Record{amplifiedHeld(1, 1), amplifiedHeld(2, 2), amplifiedHeld(3, 3)}),
		Outcomes: []stageevidence.Outcome{
			{ID: 1, Shipped: true, RoutesMeasured: true},
			{ID: 2, Shipped: true},
			{ID: 3, Shipped: true, RoutesMeasured: true},
		},
	}
	got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor("held_proposals.stage"))
	if got.Ready || got.NotReady != stageevidence.ReasonConflictsUnmeasured {
		t.Errorf("mixed outcomes = ready %v not_ready %q, want not ready %q", got.Ready, got.NotReady, stageevidence.ReasonConflictsUnmeasured)
	}
}

func TestAmplifiedAllMeasuredOutcomesLeaveNoReason(t *testing.T) {
	t.Parallel()
	chain := stageevidence.Chain{
		Records: linked([]stageevidence.Record{amplifiedHeld(1, 1), amplifiedHeld(2, 2)}),
		Outcomes: []stageevidence.Outcome{
			{ID: 1, Shipped: true, RoutesMeasured: true},
			{ID: 2, Shipped: true, RoutesMeasured: true},
		},
	}
	got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor("held_proposals.stage"))
	if got.NotReady != "" {
		t.Errorf("all measured not_ready = %q, want empty", got.NotReady)
	}
}

func TestAmplifiedUnmeasuredOutcomeOnNonConflictKeyNeverSetsReason(t *testing.T) {
	t.Parallel()
	for _, key := range []string{stageKey, "inbox.curator.stage", "gc.sentinel.hold.stage"} {
		rec := stageevidence.Record{Key: key, Version: currentVersion, Kind: stageevidence.KindDecision, ID: 1, Seq: 1, Fields: map[string]string{"would_hold": "true"}}
		chain := stageevidence.Chain{
			Records:  linked([]stageevidence.Record{rec}),
			Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}},
		}
		got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor(key))
		if got.NotReady != "" {
			t.Errorf("key %q not_ready = %q, want empty", key, got.NotReady)
		}
	}
}

func TestAmplifiedStaleVersionRecordIsUncountedAndSetsNoReason(t *testing.T) {
	t.Parallel()
	stale := amplifiedHeld(1, 1)
	stale.Version = "old-version"
	chain := stageevidence.Chain{
		Records:  linked([]stageevidence.Record{stale}),
		Outcomes: []stageevidence.Outcome{{ID: 1, Shipped: true}},
	}
	got := stageevidence.Score(chain, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor("held_proposals.stage"))
	if got.Ready || got.NotReady != "" {
		t.Errorf("stale unmeasured record = ready %v not_ready %q, want not ready with no reason", got.Ready, got.NotReady)
	}
}

func TestAmplifiedEmptyChainHasNoReason(t *testing.T) {
	t.Parallel()
	got := stageevidence.Score(stageevidence.Chain{}, currentVersion, gatecontract.Promotion{MinDecisions: 1}, stageevidence.ScorerFor("held_proposals.stage"))
	if got.Ready || got.NotReady != "" {
		t.Errorf("empty chain = ready %v not_ready %q, want not ready with no reason", got.Ready, got.NotReady)
	}
}

func TestAmplifiedJSONTagsOmitZeroValuesAndNameReason(t *testing.T) {
	t.Parallel()
	zero, err := json.Marshal(stageevidence.Result{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(zero), "not_ready") {
		t.Errorf("zero Result JSON %s names not_ready, want omitted", zero)
	}
	set, err := json.Marshal(stageevidence.Result{NotReady: stageevidence.ReasonConflictsUnmeasured})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(set, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["not_ready"] != "conflicts_unmeasured" {
		t.Errorf("Result JSON not_ready = %v, want conflicts_unmeasured", decoded["not_ready"])
	}
	out, err := json.Marshal(stageevidence.Outcome{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "routes_measured") {
		t.Errorf("zero Outcome JSON %s names routes_measured, want omitted", out)
	}
	out, err = json.Marshal(stageevidence.Outcome{ID: 1, RoutesMeasured: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"routes_measured":true`) {
		t.Errorf("Outcome JSON %s lacks routes_measured:true", out)
	}
}
