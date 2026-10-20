package wave_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestMeasureFacts_CountsTheOptimisticAdmissionsAndTheirFailuresInsideTheRunWindow(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	optimistic, failed := string(orchestrator.CodeAdmissionOptimistic), string(orchestrator.CodeOptimisticAdmissionFailed)
	p.write(".evolve/signals.ndjson",
		eventAt(-1, "loop.wave", optimistic, "")+
			eventAt(2, "loop.wave", optimistic, "")+
			eventAt(3, "loop.wave", optimistic, "")+
			eventAt(20, "loop.wave", failed, "")+
			eventAt(61, "loop.wave", failed, ""))
	next := wave.Record{Number: 6, StartedAt: clock(60)}
	loop, _ := wave.ReadLoopSignals(p.root)

	facts, _ := wave.MeasureFacts(wave.Record{Number: 5, StartedAt: clock(0)}, &next, nil, loop, 4)

	if facts.OptimisticAdmissions != 2 || facts.OptimisticFailures != 1 {
		t.Errorf("facts = %+v, want 2 optimistic admissions and 1 failure inside the run", facts)
	}
}
