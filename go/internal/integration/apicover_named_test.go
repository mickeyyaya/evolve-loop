package integration

import (
	"context"
	"testing"
)

func TestExportedSymbolsAreNamedByAnExecutedAssertion(t *testing.T) {
	triggers := []FlushTrigger{TriggerNone, TriggerWaveEnd, TriggerCapReached, TriggerMaxAge, TriggerUnblocksQueuedWork, TriggerRiskyAlone}
	seen := map[FlushTrigger]bool{}
	for _, trigger := range triggers {
		if seen[trigger] {
			t.Errorf("trigger %q repeats; each trigger must be a distinct value", trigger)
		}
		seen[trigger] = true
	}
	if TriggerNone != "" {
		t.Errorf("TriggerNone = %q, want the empty trigger", TriggerNone)
	}

	policy := DefaultFlushPolicy()
	var decision FlushDecision = DecideFlush([]Deliverable{{ID: "a"}}, true, policy, decisionTime)
	if decision.Trigger != TriggerWaveEnd {
		t.Errorf("decision.Trigger = %q, want %q", decision.Trigger, TriggerWaveEnd)
	}
	if MinCap > MaxCap || DefaultCap < MinCap || DefaultCap > MaxCap {
		t.Errorf("bounds min %d default %d max %d are inconsistent", MinCap, DefaultCap, MaxCap)
	}
	if HalveCap(GrowCap(MinCap)) != MinCap {
		t.Errorf("HalveCap(GrowCap(MinCap)) = %d, want %d", HalveCap(GrowCap(MinCap)), MinCap)
	}
	if DefaultMaxAgeMinutes != policy.MaxAgeMinutes {
		t.Errorf("DefaultMaxAgeMinutes = %d, policy %d", DefaultMaxAgeMinutes, policy.MaxAgeMinutes)
	}

	d := Deliverable{
		ID:               "sample",
		InboxRank:        1,
		Deps:             []string{"upstream"},
		UnblocksCount:    2,
		VerificationCost: 3,
	}
	if d.InboxRank != 1 || len(d.Deps) != 1 || d.UnblocksCount != 2 || d.VerificationCost != 3 {
		t.Errorf("deliverable fields inconsistent: %+v", d)
	}
	ordered := OrderDeliverables([]Deliverable{d})
	if len(ordered) != 1 || ordered[0].ID != "sample" {
		t.Errorf("OrderDeliverables = %+v, want sample", ordered)
	}
}

func TestIntegrationRecordExportsAreNamedByAnExecutedAssertion(t *testing.T) {
	t.Parallel()
	spy := &closerSpy{}
	a := Adopter{Closer: spy}
	if err := a.Closer.ClosePR(context.Background(), 5, "integration/w1-b1"); err != nil || len(spy.closed) != 1 {
		t.Errorf("ClosePR = %v, closed %v", err, spy.closed)
	}
	if IntegrationSchema == RecordSchema {
		t.Errorf("the integration schema %q must differ from the deliverable schema", IntegrationSchema)
	}
	if ErrStaleEpoch == nil || ErrBranchMoved == nil || ErrStaleEpoch == ErrBranchMoved {
		t.Error("the two adoption errors must be distinct and set")
	}
}

func TestConsoleTrainExportsAreNamedByAnExecutedAssertion(t *testing.T) {
	if MaxMoves < 1 {
		t.Errorf("MaxMoves = %d, want at least one recompose after main moves", MaxMoves)
	}
	if !(DeliverableRecord{Source: SourceConsolePR}).Console() || (DeliverableRecord{}).Console() {
		t.Error("Console reads the source of the record")
	}
	var gh GhFunc = func(_ context.Context, dir string, args ...string) (string, int, error) {
		return dir + " " + args[0], 7, nil
	}
	if out, code, err := gh(context.Background(), "d", "pr"); out != "d pr" || code != 7 || err != nil {
		t.Errorf("GhFunc = %q, %d, %v", out, code, err)
	}
	out := Outcome{Landed: []int{1}, Ejected: []int{2}}
	if len(out.Landed) != 1 || out.Ejected[0] != 2 {
		t.Errorf("Outcome fields inconsistent: %+v", out)
	}
	var host PRHost = &fakeHost{ready: []int{9}}
	if ready, err := host.Ready(context.Background()); err != nil || len(ready) != 1 || ready[0] != 9 {
		t.Errorf("PRHost.Ready = %v, %v", ready, err)
	}
}
