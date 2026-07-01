package panestream

// signalcenter_parallelevaluate_test.go — RED/regression tests for cycle-433
// slice S5, Task 1 (s5-parallelevaluate-stress-race): a ParallelEvaluate-style
// mixed-op stress harness on ONE shared *SignalCenter, proving the RWMutex
// model (ADR-0068 Option C) is race-clean under many concurrent, DISTINCT
// session producers plus concurrent Aggregate/Busy/Changed readers plus
// concurrent RegisterHandler calls — not merely the ≥8 single-op producers
// the existing signalcenter_test.go / signalcenter_busychange_test.go cover.
//
// TDD contract: written against the ALREADY-SHIPPED SignalCenter (S2-S4, on
// main). No production change is required for this task — it is expected to
// run GREEN today (pre-existing GREEN; see test-report.md). Its job is to PIN
// the concurrency invariant so Task 2's evidence-driven sharding decision
// (implement per-session locks, or record single-mutex-sufficient) cannot
// silently regress correctness.
// DO NOT MODIFY THESE TESTS — any Task 2 refactor must keep them GREEN
// unmodified, under EITHER branch of the sharding decision.

import (
	"fmt"
	"sync"
	"testing"
)

// validAggregateStates is the complete, documented Aggregate() return set
// (ADR-0068 aggregation rule + ADR-0070): 0 (empty center, no observations) plus
// the five LivenessState values. Any other value is a spec violation.
var validAggregateStates = map[LivenessState]bool{
	0:                       true,
	LivenessIdle:            true,
	LivenessBusyButStagnant: true,
	LivenessHung:            true,
	LivenessConverging:      true,
	LivenessExhausted:       true,
}

// TestSignalCenter_ParallelEvaluateStress_MixedOpsRaceClean (AC1/AC2/AC4,
// -race): simulates the ParallelEvaluate end-state — ≥16 concurrent, DISTINCT
// session producers each driving ≥100 Observe cycles on ONE shared center,
// alongside concurrent Aggregate/Busy/Changed readers and concurrent
// RegisterHandler calls, all overlapping. Must be race-clean under
// `go test -race`.
//
// This is a REAL guard, not a "did not panic" no-op, in two ways:
//  1. The reader goroutine asserts the documented Aggregate() invariant on
//     EVERY call, not merely that it returned without panicking.
//  2. The post-condition below directly inspects the (white-box, same
//     package) sessions map for lost or duplicated entries — an
//     unsynchronized map write under concurrency does not fail softly, it
//     crashes the process outright ("fatal error: concurrent map writes"),
//     so reaching the post-condition already proves synchronized map access;
//     the length check additionally proves no entries were lost.
func TestSignalCenter_ParallelEvaluateStress_MixedOpsRaceClean(t *testing.T) {
	const numProducers = 16
	const observesPerProducer = 100
	const readerIterations = 200

	sc := NewSignalCenter()
	p := Profiles["claude"]

	var wg sync.WaitGroup

	// Producers: each drives its OWN distinct session key through many
	// Observe cycles — the shape that exposes global-lock serialization of
	// independent sessions (KF2/H1 in scout-report.md).
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("pe-sess-%d", id)
			for j := 0; j < observesPerProducer; j++ {
				content := fmt.Sprintf("⏺ producer-%d tick-%d\n❯ \n", id, j)
				sc.Observe(key, content, p)
			}
		}(i)
	}

	// Concurrent RegisterHandler calls, overlapping the producers above —
	// exercises the registry-write path under contention (ADR-0068 registry
	// map, guarded by the same mutex as the sessions map).
	const numHandlers = 4
	for i := 0; i < numHandlers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("pe-handler-%d", id)
			sc.RegisterHandler(name, func() LivenessProbe { return NewDefaultDetector(0) })
		}(i)
	}

	// Reader: concurrent Aggregate/Busy/Changed on the producers' keys while
	// they are still being written. Asserts the Aggregate() invariant on
	// every call.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < readerIterations; i++ {
			state := sc.Aggregate()
			if !validAggregateStates[state] {
				t.Errorf("Aggregate() returned invalid state %v under concurrency, want one of {0,Idle,BusyButStagnant,Hung,Converging}", state)
				return
			}
			key := fmt.Sprintf("pe-sess-%d", i%numProducers)
			_ = sc.Busy(key)
			_ = sc.Changed(key)
		}
	}()

	wg.Wait()
	<-readerDone

	// Post-condition (white-box): every distinct producer key must have been
	// recorded exactly once — no lost or duplicated entries under
	// concurrency. Safe to read sc.sessions directly without the mutex here:
	// wg.Wait()/<-readerDone already establish happens-before with every
	// writer goroutine.
	if got := len(sc.sessions); got != numProducers {
		t.Errorf("after stress: len(sc.sessions) = %d, want %d (a session key was lost or duplicated under concurrency)", got, numProducers)
	}
}

// TestSignalCenter_ObserveAggregateSameKeyRaceClean (BA2 guard, -race):
// hammers Observe against Aggregate/Busy/Changed on the SAME session key
// concurrently — the shape that would surface a torn read (BA2, scout-report
// Beyond-the-Ask Hypotheses) if a future sharding refactor moved
// ss.last/ss.busy/ss.changed/ss.clean mutation under a per-session lock
// without updating Aggregate/Busy/Changed to read under that SAME lock.
// Race-clean today under the single RWMutex; must stay race-clean after
// Task 2, regardless of which branch (single-lock vs. sharded) is chosen.
func TestSignalCenter_ObserveAggregateSameKeyRaceClean(t *testing.T) {
	const iterations = 500

	sc := NewSignalCenter()
	p := Profiles["claude"]
	key := "shared-key"
	sc.Observe(key, "⏺ priming\n❯ \n", p) // create the key before racing reads against it

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			sc.Observe(key, fmt.Sprintf("⏺ tick-%d\n❯ \n", i), p)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			state := sc.Aggregate()
			if !validAggregateStates[state] {
				t.Errorf("Aggregate() returned invalid state %v during same-key concurrent Observe/Aggregate, want one of {0,Idle,BusyButStagnant,Hung,Converging}", state)
				return
			}
			_ = sc.Busy(key)
			_ = sc.Changed(key)
		}
	}()
	wg.Wait()
}
