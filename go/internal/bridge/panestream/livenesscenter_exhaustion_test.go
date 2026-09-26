package panestream

import (
	"fmt"
	"sync"
	"testing"
)

// Names the LivenessHandler type for apicover.
var _ LivenessHandler = func(LivenessEvent) {}

func TestSignalCenter_ExhaustedDominatesAggregate(t *testing.T) {
	sc := NewLivenessCenter()
	walled := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota (exceeded|reached)`}
	working := PaneProfile{Name: "codex"}
	sc.Observe("working", "line1\n", working)
	sc.Observe("working", "line1\nline2\n", working)
	sc.Observe("walled", "⚠ Individual quota reached. Resets in 52h\n", walled)

	if got := sc.Aggregate(); got != LivenessExhausted {
		t.Fatalf("Aggregate=%v, want LivenessExhausted (a wall dominates every other state)", got)
	}
}

func TestSignalCenter_DispatchesExhaustionToHandler(t *testing.T) {
	sc := NewLivenessCenter()
	var events []LivenessEvent
	sc.RegisterLivenessHandler(func(ev LivenessEvent) { events = append(events, ev) })

	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	sc.Observe("s1", "working normally\n", p)
	sc.Observe("s1", "⚠ quota reached\n", p)
	sc.Observe("s1", "⚠ quota reached now\n", p) // still Exhausted: no re-dispatch

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

func TestSignalCenter_RegisterNilSignalHandler_NoOp(t *testing.T) {
	sc := NewLivenessCenter()
	sc.RegisterLivenessHandler(nil)
	p := PaneProfile{Name: "agy", ExhaustedRegex: `(?i)quota reached`}
	sc.Observe("s1", "⚠ quota reached\n", p)
}

func TestSignalCenter_ConcurrentDispatchAndRegister_RaceClean(t *testing.T) {
	sc := NewLivenessCenter()
	var mu sync.Mutex
	seen := 0
	sc.RegisterLivenessHandler(func(ev LivenessEvent) {
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
		go func() { defer wg.Done(); sc.RegisterLivenessHandler(func(LivenessEvent) {}) }()
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
