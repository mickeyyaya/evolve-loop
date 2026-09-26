package panestream

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// fixedProbe always returns a predetermined state.
type fixedProbe struct {
	state LivenessState
	conf  float64
}

func (f *fixedProbe) Assess(_ string, _ PaneProfile) (LivenessState, float64) {
	return f.state, f.conf
}

func TestAmp_SignalCenter_AggregationPriority_HungBeatsBusyStagnant(t *testing.T) {
	sc := NewLivenessCenter()

	sc.RegisterHandler("hung-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessHung, conf: 0.8}
	})
	sc.RegisterHandler("stagnant-probe", func() LivenessProbe {
		return &fixedProbe{state: LivenessBusyButStagnant, conf: 0.6}
	})

	hungP := PaneProfile{Name: "hung-probe", BoundaryMarker: "$"}
	stagP := PaneProfile{Name: "stagnant-probe", BoundaryMarker: "$"}

	sc.Observe("sess-hung", "content\n$ \n", hungP)
	sc.Observe("sess-hung", "content\n$ \n", hungP)
	sc.Observe("sess-stagnant", "content\n$ \n", stagP)
	sc.Observe("sess-stagnant", "content\n$ \n", stagP)

	got := sc.Aggregate()
	if got != LivenessHung {
		t.Errorf("Hung+BusyStagnant: Aggregate()=%v, want LivenessHung (priority: Hung > BusyStagnant per ADR-0068)", got)
	}
}

func TestAmp_SignalCenter_AggregationPriority_ConvergingBeatsHung(t *testing.T) {
	sc := NewLivenessCenter()

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

func TestAmp_SignalCenter_AggregationPriority_ConvergingWinsAll(t *testing.T) {
	sc := NewLivenessCenter()

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

func TestAmp_SignalCenter_FactoryCalledOncePerSession(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestAmp_SignalCenter_SessionIsolation(t *testing.T) {
	sc := NewLivenessCenter()
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

	if c := atomic.LoadInt64(&convCalls); c != 1 {
		t.Errorf("session-A: conv factory called %d times, want 1", c)
	}
	if c := atomic.LoadInt64(&stalCalls); c != 1 {
		t.Errorf("session-B: stag factory called %d times, want 1", c)
	}

	got := sc.Aggregate()
	if got != LivenessConverging {
		t.Errorf("session isolation aggregate: got %v, want LivenessConverging (sess-A dominates)", got)
	}
}

func TestAmp_SignalCenter_EmptySessionKey(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestAmp_SignalCenter_EmptyRendered(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestAmp_SignalCenter_LargeNumSessions(t *testing.T) {
	sc := NewLivenessCenter()
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
	if got == 0 {
		t.Errorf("200 sessions Aggregate()=0 (unset); want a valid LivenessState")
	}
}

func TestAmp_SignalCenter_MultipleReadersConcurrent(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

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

func TestAmp_SignalCenter_SingleObservationPerSession(t *testing.T) {
	sc := NewLivenessCenter()
	p := Profiles["claude"]

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("single-observation Aggregate() panicked: %v", r)
		}
	}()

	sc.Observe("sess-single", "content\n❯ \n", p)
	_ = sc.Aggregate() // must not panic; state is implementation-defined
}

func TestAmp_SignalCenter_RegisteredHandlerOverridesBuiltin(t *testing.T) {
	sc := NewLivenessCenter()
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

func TestAmp_SignalCenter_AllIdleSessionsAggregateIdle(t *testing.T) {
	sc := NewLivenessCenter()

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
