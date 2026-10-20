package wave_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func optimisticFacts(t *testing.T, stream string) wavemeasure.Facts {
	t.Helper()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson", stream)
	next := wave.Record{Number: 6, StartedAt: clock(60)}
	loop, _ := wave.ReadLoopSignals(p.root)
	facts, _ := wave.MeasureFacts(wave.Record{Number: 5, StartedAt: clock(0)}, &next, nil, loop, 4)
	return facts
}

func TestMeasureFacts_CountsTheOptimisticAdmissionsAndTheirFailuresInsideTheRunWindow(t *testing.T) {
	t.Parallel()
	optimistic, failed := string(orchestrator.CodeAdmissionOptimistic), string(orchestrator.CodeOptimisticAdmissionFailed)
	facts := optimisticFacts(t,
		eventAt(2, "loop.wave", optimistic, `"todo":"a"`)+
			eventAt(3, "loop.wave", optimistic, `"todo":"b"`)+
			eventAt(4, "loop.wave", optimistic, `"todo":"a"`)+
			eventAt(20, "loop.wave", failed, `"todo":"a"`)+
			eventAt(21, "loop.wave", failed, `"todo":"a"`)+
			eventAt(61, "loop.wave", failed, `"todo":"b"`))

	if facts.OptimisticAdmissions != 2 || facts.OptimisticFailures != 1 {
		t.Errorf("facts = %+v, want 2 optimistic admissions (a relaunch of a counts once) and 1 failure inside the run", facts)
	}
}

func TestMeasureFacts_AFailureCountsOnlyWhenItsAdmissionIsCounted(t *testing.T) {
	t.Parallel()
	optimistic, failed := string(orchestrator.CodeAdmissionOptimistic), string(orchestrator.CodeOptimisticAdmissionFailed)
	facts := optimisticFacts(t,
		eventAt(-1, "loop.wave", optimistic, `"todo":"early"`)+
			eventAt(2, "loop.wave", optimistic, `"todo":"b"`)+
			eventAt(20, "loop.wave", failed, `"todo":"early"`)+
			eventAt(25, "loop.wave", failed, `"todo":"b"`)+
			eventAt(26, "loop.wave", failed, `"todo":"stranger"`)+
			eventAt(27, "loop.wave", failed, ""))

	if facts.OptimisticAdmissions != 1 || facts.OptimisticFailures != 1 {
		t.Fatalf("facts = %+v, want 1 admission and 1 failure: a lane launched before started_at (#1013) or never admitted cannot add a failure", facts)
	}
	if m := wavemeasure.Compute(facts, clock(60)); m.OptimisticFailureRate > 1 {
		t.Errorf("rate = %v, want a value in [0,1]", m.OptimisticFailureRate)
	}
}
