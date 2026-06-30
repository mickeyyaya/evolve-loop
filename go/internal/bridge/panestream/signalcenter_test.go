package panestream

import (
	"fmt"
	"sync"
	"testing"
)

// signalcenter_test.go — Behavioral tests for SignalCenter (S2, cycle 430).
// TDD contract: these tests are written BEFORE the production code (signalcenter.go).
// They compile-fail until Builder implements the SignalCenter type.
// DO NOT MODIFY THESE TESTS — Builder implements to make them GREEN.

// ── Task 1: signalcenter-facade-concurrency (S2a) ────────────────────────────

// TestSignalCenter_ObserveAndAggregate (AC1, positive):
// Observe writes a liveness signal for a session; Aggregate returns one non-zero LivenessState.
func TestSignalCenter_ObserveAndAggregate(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]
	// Prime on first call (first Assess is baseline), then provide a second
	// observation that raises ↓ tokens → ClaudeDetector reports Converging.
	sc.Observe("sess-1", "⏺ thinking\n(4s · ↓ 50 tokens)\n❯ \n", p)
	sc.Observe("sess-1", "⏺ thinking\n(4s · ↓ 100 tokens)\n❯ \n", p)
	state := sc.Aggregate()
	if state == 0 {
		t.Errorf("Aggregate after two Observes: got zero LivenessState (unset), want a valid state")
	}
}

// TestSignalCenter_AggregateIsDeterministic (AC2, semantic):
// Two consecutive calls to Aggregate with no new observations return the same value.
func TestSignalCenter_AggregateIsDeterministic(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]
	sc.Observe("sess-1", "⏺ line A\n❯ \n", p)
	sc.Observe("sess-1", "⏺ line A\n⏺ line B\n❯ \n", p)
	s1 := sc.Aggregate()
	s2 := sc.Aggregate()
	if s1 != s2 {
		t.Errorf("Aggregate not deterministic: first call=%v second call=%v", s1, s2)
	}
}

// TestSignalCenter_UnknownProfileFallsToDefault (AC3, OOD/edge):
// A profile whose Name is not in Profiles must fall back to DefaultDetector (no panic).
func TestSignalCenter_UnknownProfileFallsToDefault(t *testing.T) {
	sc := NewSignalCenter()
	unknown := PaneProfile{Name: "unknown-cli-xyz", BoundaryMarker: "$"}
	// Must not panic; DefaultDetector handles any pane content.
	sc.Observe("sess-x", "some content\n$ \n", unknown)
	sc.Observe("sess-x", "some content\nnew line\n$ \n", unknown)
	state := sc.Aggregate()
	// DefaultDetector on two observations returns a non-zero state.
	if state == 0 {
		t.Errorf("UnknownProfile: Aggregate returned zero state, expected DefaultDetector fallback to return a valid state")
	}
}

// TestSignalCenter_EmptyCenter_DefinedState (AC4, negative):
// Aggregate on a center with no observations must not panic; zero state is acceptable.
func TestSignalCenter_EmptyCenter_DefinedState(t *testing.T) {
	sc := NewSignalCenter()
	// No observations: must not panic. Return value may be zero (no sessions).
	_ = sc.Aggregate()
}

// TestSignalCenter_ParallelProducerRaceClean (AC5, -race):
// 8 concurrent producers (4 sharing a session key) + a concurrent reader must be
// race-clean. Validates RWMutex model under ParallelEvaluate-style dispatch.
func TestSignalCenter_ParallelProducerRaceClean(t *testing.T) {
	const numProducers = 8
	sc := NewSignalCenter()
	claudeProfile := Profiles["claude"]
	sharedKey := "shared-sess"

	var wg sync.WaitGroup
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("sess-%d", id)
			if id%2 == 0 {
				key = sharedKey // contention: multiple writers on same session key
			}
			sc.Observe(key, fmt.Sprintf("content-%d\n❯ \n", id), claudeProfile)
		}(i)
	}

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < 8; i++ {
			_ = sc.Aggregate()
		}
	}()

	wg.Wait()
	<-readerDone
}

// ── Task 2: signalcenter-handler-registry (S2b) ──────────────────────────────

// TestSignalCenter_RegisterHandlerRoutesNewCLI (Task2/AC1, positive):
// A handler registered for "fake-cli" must be invoked when Observe is called with
// that profile — routed through the registry, not through DetectorFor's switch.
// This is the load-bearing add-a-CLI-without-switch-edit test.
func TestSignalCenter_RegisterHandlerRoutesNewCLI(t *testing.T) {
	sc := NewSignalCenter()
	factoryCalled := false
	sc.RegisterHandler("fake-cli", func() LivenessProbe {
		factoryCalled = true
		return NewDefaultDetector(0)
	})
	fakeProfile := PaneProfile{Name: "fake-cli", BoundaryMarker: "»"}
	sc.Observe("sess-1", "content\n» \n", fakeProfile)
	if !factoryCalled {
		t.Errorf("RegisterHandler: factory not called when Observe used 'fake-cli' profile (not routed through registry)")
	}
}

// TestSignalCenter_UnregisteredProfileFallsToDefault (Task2/AC2, edge):
// A profile whose Name was never registered must fall back to DefaultDetector.
func TestSignalCenter_UnregisteredProfileFallsToDefault(t *testing.T) {
	sc := NewSignalCenter()
	p := PaneProfile{Name: "no-such-cli", BoundaryMarker: "~"}
	sc.Observe("sess-1", "content\n~ \n", p)
	sc.Observe("sess-1", "content\nnew line\n~ \n", p)
	state := sc.Aggregate()
	if state == 0 {
		t.Errorf("UnregisteredProfile: Aggregate returned zero state (expected DefaultDetector fallback to return valid state)")
	}
}

// TestSignalCenter_RegisterEmptyOrDuplicateNoPanic (Task2/AC3, negative):
// Empty-key registration and duplicate-key registration must not panic.
// Behavior (ignored vs last-write-wins) is implementation-defined; no crash is the contract.
func TestSignalCenter_RegisterEmptyOrDuplicateNoPanic(t *testing.T) {
	sc := NewSignalCenter()
	// Empty key: defined (no panic).
	sc.RegisterHandler("", func() LivenessProbe { return NewDefaultDetector(0) })
	// Duplicate key: defined (no panic); second registration may win or be dropped.
	sc.RegisterHandler("claude", func() LivenessProbe { return NewClaudeDetector(0) })
	sc.RegisterHandler("claude", func() LivenessProbe { return NewClaudeDetector(0) })
}

// TestSignalCenter_RegisterConcurrent (Task2/AC5, -race):
// Concurrent RegisterHandler + Observe must be race-clean under -race.
func TestSignalCenter_RegisterConcurrent(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sc.RegisterHandler(fmt.Sprintf("cli-%d", id), func() LivenessProbe {
				return NewDefaultDetector(0)
			})
			sc.Observe(fmt.Sprintf("sess-%d", id), fmt.Sprintf("content-%d\n❯ \n", id), p)
		}(i)
	}
	wg.Wait()
}
