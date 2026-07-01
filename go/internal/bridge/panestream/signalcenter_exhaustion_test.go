package panestream

import (
	"fmt"
	"sync"
	"testing"
)

// Compile-time conformance: a plain reaction function satisfies the SignalHandler
// type (the Observer contract consumers register via RegisterSignalHandler).
var _ SignalHandler = func(SignalEvent) {}

// signalcenter_exhaustion_test.go — the SignalCenter's exhaustion integration
// (S1): the center wraps every per-CLI probe in an ExhaustionProbe so exhaustion
// is detected through the SAME abstraction as liveness, dominates the aggregate,
// and is dispatched to registered handlers (Observer) so a reactive consumer
// (the CLI-bench) is told which session walled without polling.

// A walled session dominates the aggregate: even with a healthy Converging
// session present, any Exhausted session makes the center report Exhausted —
// the top of the winner-takes-all priority order.
func TestSignalCenter_ExhaustedDominatesAggregate(t *testing.T) {
	sc := NewSignalCenter()
	walled := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota (exceeded|reached)`}
	working := PaneProfile{Name: "codex"}
	// A converging session (new content across two frames).
	sc.Observe("working", "line1\n", working)
	sc.Observe("working", "line1\nline2\n", working)
	// A walled session.
	sc.Observe("walled", "⚠ Individual quota reached. Resets in 52h\n", walled)

	if got := sc.Aggregate(); got != LivenessExhausted {
		t.Fatalf("Aggregate=%v, want LivenessExhausted (a wall dominates every other state)", got)
	}
}

// The center dispatches a SignalEvent to every registered handler when a session
// transitions to LivenessExhausted — exactly once per transition (edge-triggered,
// not re-fired every frame while it stays walled).
func TestSignalCenter_DispatchesExhaustionToHandler(t *testing.T) {
	sc := NewSignalCenter()
	var events []SignalEvent
	sc.RegisterSignalHandler(func(ev SignalEvent) { events = append(events, ev) })

	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	sc.Observe("s1", "working normally\n", p)    // healthy
	sc.Observe("s1", "⚠ quota reached\n", p)     // transition → Exhausted (dispatch)
	sc.Observe("s1", "⚠ quota reached now\n", p) // still Exhausted (no re-dispatch)

	exhausted := 0
	for _, ev := range events {
		if ev.State == LivenessExhausted {
			if ev.SessionKey != "s1" {
				t.Errorf("exhaustion event SessionKey=%q, want s1", ev.SessionKey)
			}
			exhausted++
		}
	}
	if exhausted != 1 {
		t.Fatalf("dispatched %d exhaustion events, want exactly 1 (edge-triggered on transition)", exhausted)
	}
}

// A nil handler registration is a safe no-op (never dispatched, never panics),
// mirroring RegisterHandler's empty-name tolerance.
func TestSignalCenter_RegisterNilSignalHandler_NoOp(t *testing.T) {
	sc := NewSignalCenter()
	sc.RegisterSignalHandler(nil)
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	sc.Observe("s1", "⚠ quota reached\n", p) // must not panic
}

// Concurrent RegisterSignalHandler + distinct-session Observe must be race-free
// (-race): sc.handlers is guarded by sc.mu (snapshot under it, dispatched
// outside it); each session's state is guarded by its own ss.mu. Deterministic
// tail: every session ends on a walled frame (Aggregate == Exhausted), and the
// handler registered BEFORE any producer sees at least one exhaustion event.
func TestSignalCenter_ConcurrentDispatchAndRegister_RaceClean(t *testing.T) {
	sc := NewSignalCenter()
	var mu sync.Mutex
	seen := 0
	sc.RegisterSignalHandler(func(ev SignalEvent) {
		if ev.State == LivenessExhausted {
			mu.Lock()
			seen++
			mu.Unlock()
		}
	})
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}

	var wg sync.WaitGroup
	for h := 0; h < 3; h++ { // late registrations racing the producers
		wg.Add(1)
		go func() { defer wg.Done(); sc.RegisterSignalHandler(func(SignalEvent) {}) }()
	}
	for s := 0; s < 8; s++ { // distinct-session producers, each ending walled
		wg.Add(1)
		key := fmt.Sprintf("sess-%d", s)
		go func(k string) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				sc.Observe(k, "working normally\n", p)
				sc.Observe(k, "⚠ quota reached\n", p)
			}
		}(key)
	}
	wg.Wait()

	if got := sc.Aggregate(); got != LivenessExhausted {
		t.Errorf("Aggregate=%v, want LivenessExhausted (every session ended on a walled frame)", got)
	}
	if seen == 0 {
		t.Error("the upfront handler received no exhaustion events — dispatch under concurrency failed")
	}
}
