package panestream

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// signalcenter_amplify_test.go — Adversarial amplification for SignalCenter (cycle 430).
// Written by the Test Amplifier — black-box view, spec + ADR-0068 only.
// No implementation files read; fixedProbe injects controlled LivenessState values.
//
// Coverage gaps targeted (not reached by signalcenter_test.go AC1–AC12):
//   AMP1  Aggregation priority: Hung > BusyButStagnant (two sessions, different states)
//   AMP2  Aggregation priority: Converging > Hung (two sessions)
//   AMP3  Aggregation priority: Converging wins over all four states simultaneously
//   AMP4  Factory call frequency: called exactly once per session key (not per-Observe)
//   AMP5  Session isolation: two keys drive independent probe state machines
//   AMP6  Empty session key: no panic on Observe or Aggregate
//   AMP7  Empty rendered string: no panic (boundary for regex/string-scan detectors)
//   AMP8  Large N sessions (200): no panic, valid aggregate
//   AMP9  Multiple concurrent readers: -race clean (existing test has only 1 reader)
//   AMP10 Single observation per session: Aggregate doesn't panic (half-primed probe)
//   AMP11 RegisterHandler overrides built-in "claude" profile (registry-first OCP seam)
//   AMP12 All-Idle sessions: Aggregate returns LivenessIdle, not zero

// fixedProbe is a LivenessProbe that always returns a predetermined state.
// Uses the exported LivenessProbe interface only — no implementation dependency.
type fixedProbe struct {
	state LivenessState
	conf  float64
}

func (f *fixedProbe) Assess(_ string, _ PaneProfile) (LivenessState, float64) {
	return f.state, f.conf
}

// TestAmp_SignalCenter_AggregationPriority_HungBeatsBusyStagnant (AMP1):
// When one session is Hung and another is BusyButStagnant, Aggregate must return
// LivenessHung. Validates priority level 2 > level 3 from ADR-0068 aggregation rule.
func TestAmp_SignalCenter_AggregationPriority_HungBeatsBusyStagnant(t *testing.T) {
	sc := NewSignalCenter()

	sc.RegisterHandler("hung-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessHung, conf: 0.8}
	})
	sc.RegisterHandler("stagnant-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessBusyButStagnant, conf: 0.6}
	})

	hungP := PaneProfile{Name: "hung-probe", BoundaryMarker: "$"}
	stagP := PaneProfile{Name: "stagnant-probe", BoundaryMarker: "$"}

	// Two observations each to match the established test pattern (prime + assess).
	sc.Observe("sess-hung", "content\n$ \n", hungP)
	sc.Observe("sess-hung", "content\n$ \n", hungP)
	sc.Observe("sess-stagnant", "content\n$ \n", stagP)
	sc.Observe("sess-stagnant", "content\n$ \n", stagP)

	got := sc.Aggregate()
	if got != LivenessHung {
		t.Errorf("Hung+BusyStagnant: Aggregate()=%v, want LivenessHung (priority: Hung > BusyStagnant per ADR-0068)", got)
	}
}

// TestAmp_SignalCenter_AggregationPriority_ConvergingBeatsHung (AMP2):
// When one session is Converging and another is Hung, Aggregate must return
// LivenessConverging. Validates priority level 1 > level 2 from ADR-0068.
func TestAmp_SignalCenter_AggregationPriority_ConvergingBeatsHung(t *testing.T) {
	sc := NewSignalCenter()

	sc.RegisterHandler("conv-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessConverging, conf: 0.95}
	})
	sc.RegisterHandler("hung-probe2", func() LivenessProbe {
		return &fixedProbe{state: LivenessHung, conf: 0.8}
	})

	sc.Observe("sess-conv", "content\n$ \n", PaneProfile{Name: "conv-probe", BoundaryMarker: "$"})
	sc.Observe("sess-conv", "content\n$ \n", PaneProfile{Name: "conv-probe", BoundaryMarker: "$"})
	sc.Observe("sess-hung", "content\n$ \n", PaneProfile{Name: "hung-probe2", BoundaryMarker: "$"})
	sc.Observe("sess-hung", "content\n$ \n", PaneProfile{Name: "hung-probe2", BoundaryMarker: "$"})

	got := sc.Aggregate()
	if got != LivenessConverging {
		t.Errorf("Converging+Hung: Aggregate()=%v, want LivenessConverging (priority: Converging > Hung per ADR-0068)", got)
	}
}

// TestAmp_SignalCenter_AggregationPriority_ConvergingWinsAll (AMP3):
// Four sessions cover all four LivenessState values; Converging must win.
// Validates the full priority ordering (Converging > Hung > BusyStagnant > Idle)
// in a single assertion, guarding the complete ADR-0068 aggregation rule.
func TestAmp_SignalCenter_AggregationPriority_ConvergingWinsAll(t *testing.T) {
	sc := NewSignalCenter()

	states := []struct {
		name  string
		state LivenessState
	}{
		{"conv-all", LivenessConverging},
		{"hung-all", LivenessHung},
		{"stag-all", LivenessBusyButStagnant},
		{"idle-all", LivenessIdle},
	}
	for _, s := range states {
		state := s.state // loop-variable capture
		sc.RegisterHandler(s.name, func() LivenessProbe {
			return &fixedProbe{state: state, conf: 0.9}
		})
		p := PaneProfile{Name: s.name, BoundaryMarker: "$"}
		sc.Observe("sess-"+s.name, "content\n$ \n", p)
		sc.Observe("sess-"+s.name, "content\n$ \n", p)
	}

	got := sc.Aggregate()
	if got != LivenessConverging {
		t.Errorf("all four states: Aggregate()=%v, want LivenessConverging (top priority)", got)
	}
}

// TestAmp_SignalCenter_FactoryCalledOncePerSession (AMP4):
// A registered factory must be called exactly once per unique session key —
// not once per Observe call. Calling it on every Observe would reset stateful
// detector history (stall counts, peak tokens) — a correctness regression.
func TestAmp_SignalCenter_FactoryCalledOncePerSession(t *testing.T) {
	sc := NewSignalCenter()
	var callCount int64

	sc.RegisterHandler("counting-cli", func() LivenessProbe {
		atomic.AddInt64(&callCount, 1)
		return &fixedProbe{state: LivenessConverging, conf: 0.9}
	})

	p := PaneProfile{Name: "counting-cli", BoundaryMarker: "$"}
	for i := 0; i < 10; i++ {
		sc.Observe("same-session", fmt.Sprintf("content-%d\n$ \n", i), p)
	}

	got := atomic.LoadInt64(&callCount)
	if got != 1 {
		t.Errorf("factory called %d times for 10 Observe calls on 1 session key; want 1 (per-session, not per-Observe)", got)
	}
}

// TestAmp_SignalCenter_SessionIsolation (AMP5):
// Two different session keys must get independent probe instances.
// A shared-probe bug (single probe reused across all sessions) would corrupt
// the stall-counter state of one session when the other is observed.
func TestAmp_SignalCenter_SessionIsolation(t *testing.T) {
	sc := NewSignalCenter()
	var convCalls, stalCalls int64

	sc.RegisterHandler("conv-iso", func() LivenessProbe {
		atomic.AddInt64(&convCalls, 1)
		return &fixedProbe{state: LivenessConverging, conf: 0.95}
	})
	sc.RegisterHandler("stag-iso", func() LivenessProbe {
		atomic.AddInt64(&stalCalls, 1)
		return &fixedProbe{state: LivenessBusyButStagnant, conf: 0.6}
	})

	convP := PaneProfile{Name: "conv-iso", BoundaryMarker: "$"}
	stagP := PaneProfile{Name: "stag-iso", BoundaryMarker: "$"}

	sc.Observe("sess-A", "content\n$ \n", convP)
	sc.Observe("sess-A", "content\n$ \n", convP)
	sc.Observe("sess-B", "content\n$ \n", stagP)
	sc.Observe("sess-B", "content\n$ \n", stagP)

	// Each session must have created exactly one probe.
	if c := atomic.LoadInt64(&convCalls); c != 1 {
		t.Errorf("session-A: conv factory called %d times, want 1", c)
	}
	if c := atomic.LoadInt64(&stalCalls); c != 1 {
		t.Errorf("session-B: stag factory called %d times, want 1", c)
	}

	// Aggregate accounts for both sessions; Converging beats BusyStagnant.
	got := sc.Aggregate()
	if got != LivenessConverging {
		t.Errorf("session isolation aggregate: got %v, want LivenessConverging (sess-A dominates)", got)
	}
}

// TestAmp_SignalCenter_EmptySessionKey (AMP6):
// Observe and Aggregate with "" as session key must not panic.
// Maps in Go allow empty-string keys; implementations that validate session key
// non-emptiness with a panic would violate the no-panic contract.
func TestAmp_SignalCenter_EmptySessionKey(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("empty session key caused panic: %v", r)
		}
	}()

	sc.Observe("", "content\n❯ \n", p)
	sc.Observe("", "more content\n❯ \n", p)
	_ = sc.Aggregate()
}

// TestAmp_SignalCenter_EmptyRendered (AMP7):
// Observe with an empty rendered string must not panic.
// Regex-based detectors (ClaudeDetector, DetectorFor) call strings.Split or
// FindStringSubmatch on the rendered pane — empty input is the boundary that
// causes index-out-of-bounds in naive implementations.
func TestAmp_SignalCenter_EmptyRendered(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("empty rendered string caused panic: %v", r)
		}
	}()

	sc.Observe("sess-empty", "", p)
	sc.Observe("sess-empty", "", p)
	_ = sc.Aggregate()
}

// TestAmp_SignalCenter_LargeNumSessions (AMP8):
// 200 unique session keys — no panic, Aggregate returns a defined state.
// Guards against map-resize crashes, O(n²) aggregation, or per-map-entry
// goroutine-leak bugs that only manifest at scale.
func TestAmp_SignalCenter_LargeNumSessions(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]

	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("sess-%04d", i)
		sc.Observe(key, "output line\n❯ \n", p)
		sc.Observe(key, "output line\n❯ \n", p)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("large N sessions caused panic: %v", r)
		}
	}()

	got := sc.Aggregate()
	// 200 primed sessions must yield some defined state (not zero).
	if got == 0 {
		t.Errorf("200 sessions Aggregate()=0 (unset); want a valid LivenessState")
	}
}

// TestAmp_SignalCenter_MultipleReadersConcurrent (AMP9):
// Four concurrent Aggregate() readers and four concurrent Observe writers must be
// race-clean. The baseline test (AC5) has only 1 reader goroutine; 4 readers
// validates that RWMutex allows simultaneous reader acquisition without deadlock.
func TestAmp_SignalCenter_MultipleReadersConcurrent(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]

	// Seed some sessions so Aggregate has meaningful work to do.
	for i := 0; i < 4; i++ {
		sc.Observe(fmt.Sprintf("seed-%d", i), "seed content\n❯ \n", p)
	}

	const numReaders = 4
	const numWriters = 4

	var wg sync.WaitGroup

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = sc.Aggregate()
			}
		}()
	}

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				sc.Observe(fmt.Sprintf("writer-sess-%d", id),
					fmt.Sprintf("content-%d\n❯ \n", j), p)
			}
		}(i)
	}

	wg.Wait()
}

// TestAmp_SignalCenter_SingleObservationPerSession (AMP10):
// A session with only ONE Observe call (detector primed, not yet assessed) must
// not cause Aggregate to panic. The return value is implementation-defined
// (may be zero or a valid state depending on whether DetectorFor uses prime-on-first
// or prime+assess models) — panic-free is the only hard contract here.
func TestAmp_SignalCenter_SingleObservationPerSession(t *testing.T) {
	sc := NewSignalCenter()
	p := Profiles["claude"]

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("single-observation Aggregate() panicked: %v", r)
		}
	}()

	sc.Observe("sess-single", "content\n❯ \n", p)
	_ = sc.Aggregate() // must not panic; state is implementation-defined
}

// TestAmp_SignalCenter_RegisteredHandlerOverridesBuiltin (AMP11):
// RegisterHandler("claude", factory) must take priority over the built-in
// DetectorFor routing for the "claude" profile. Tests the OCP seam: the registry
// is consulted FIRST, before DetectorFor. This allows add-a-CLI and override-a-CLI
// without editing DetectorFor.
func TestAmp_SignalCenter_RegisteredHandlerOverridesBuiltin(t *testing.T) {
	sc := NewSignalCenter()
	overrideCalled := false

	sc.RegisterHandler("claude", func() LivenessProbe {
		overrideCalled = true
		return &fixedProbe{state: LivenessConverging, conf: 0.99}
	})

	sc.Observe("sess-override", "content\n❯ \n", Profiles["claude"])
	if !overrideCalled {
		t.Errorf("RegisterHandler(\"claude\", ...) was not called when Observe used Profiles[\"claude\"]; registry must take priority over DetectorFor (OCP seam)")
	}
}

// TestAmp_SignalCenter_AllIdleSessionsAggregateIdle (AMP12):
// When all sessions are at LivenessIdle, Aggregate must return LivenessIdle —
// not 0 (zero is reserved for the empty-center case per ADR-0068 rule 5).
// This validates the distinction between "no sessions" (zero) and "all-idle sessions"
// (LivenessIdle), which an off-by-one in the priority sweep could collapse.
func TestAmp_SignalCenter_AllIdleSessionsAggregateIdle(t *testing.T) {
	sc := NewSignalCenter()

	sc.RegisterHandler("idle-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessIdle, conf: 0.7}
	})

	idleP := PaneProfile{Name: "idle-probe", BoundaryMarker: "$"}
	sc.Observe("sess-idle-1", "content\n$ \n", idleP)
	sc.Observe("sess-idle-1", "content\n$ \n", idleP)
	sc.Observe("sess-idle-2", "content\n$ \n", idleP)
	sc.Observe("sess-idle-2", "content\n$ \n", idleP)

	got := sc.Aggregate()
	if got != LivenessIdle {
		t.Errorf("all-Idle sessions: Aggregate()=%v, want LivenessIdle (not 0; zero is reserved for empty-center only)", got)
	}
}
