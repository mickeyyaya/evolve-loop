package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestPrepareIteration_ABrakeStopRecordsALoopHaltSignalWithTheStopReason(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	if err := os.WriteFile(paths.LoopStopPath(evolveDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	u13StubRefresh(t, func() bool { return false })
	center := signalcenter.New()
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: io.Discard}
	b.deps.Signals = center
	b.deps.Storage = &fixtures.FakeStorage{}
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)

	center.Flush()
	if d.flow != batchStopIterations || b.result.StopReason != loopOperatorBrakeStop {
		t.Fatalf("flow=%v stop=%q, want the brake to stop the iterations", d.flow, b.result.StopReason)
	}
	var halts []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Kind == signalcenter.KindLoopHalt && e.Fields["stop_reason"] == loopOperatorBrakeStop {
			halts = append(halts, e)
		}
	}
	if len(halts) != 1 {
		t.Fatalf("signals = %+v, want exactly one loop.halt with stop_reason %s so the wave latency check can leave this wave out", center.Recent(), loopOperatorBrakeStop)
	}
}

func TestLoopHaltKind_IsAProjectStreamKindSoTheLatencyCheckCanReadTheBrakeAndTheStopReason(t *testing.T) {
	halt := signalcenter.Event{Kind: signalcenter.KindLoopHalt}
	if !legacyProjectRecord(halt) {
		t.Fatalf("a loop.halt event with no cycle is not written to the project stream: legacyProjectRecord = false")
	}
}
