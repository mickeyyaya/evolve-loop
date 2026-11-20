package looppreflight

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

const pauseTestTotal = uint64(100) << 30

func pauseGuard(path string, pcts ...float64) *DiskGuard {
	idx := 0
	return NewDiskGuard(path, DefaultDiskGuardConfig(), func(string) (DiskUsage, error) {
		pct := pcts[min(idx, len(pcts)-1)]
		idx++
		return DiskUsage{FreeBytes: uint64(pct * float64(pauseTestTotal) / 100), TotalBytes: pauseTestTotal}, nil
	}, io.Discard)
}

type fakeClock struct{ at time.Time }

func (c *fakeClock) now() time.Time { return c.at }

func (c *fakeClock) wait(step time.Duration) func(context.Context) error {
	return func(context.Context) error {
		c.at = c.at.Add(step)
		return nil
	}
}

func TestDiskGuard_MeasureCarriesValuesAsData(t *testing.T) {
	g := pauseGuard("/vol/x", 2, 50)
	st := g.Measure()
	if !st.Paused || st.FreePct != 2 || st.PausePct != 5 || st.Path != "/vol/x" {
		t.Fatalf("Status at 2%% = %+v, want paused free=2 pause=5 path=/vol/x", st)
	}
	if st = g.Measure(); st.Paused || st.FreePct != 50 {
		t.Fatalf("Status at 50%% = %+v, want not paused free=50", st)
	}
}

func TestDiskGuard_MeasureKeepsLastMeasureWhenProbeFails(t *testing.T) {
	calls := 0
	g := NewDiskGuard("/v", DefaultDiskGuardConfig(), func(string) (DiskUsage, error) {
		calls++
		if calls == 1 {
			return DiskUsage{FreeBytes: 3 << 30, TotalBytes: pauseTestTotal}, nil
		}
		return DiskUsage{}, errors.New("probe failed")
	}, io.Discard)
	g.Measure()
	st := g.Measure()
	if !st.Paused || st.FreePct != 3 {
		t.Fatalf("Status after a failed probe = %+v, want paused free=3 (last measure)", st)
	}
}

func TestRunCycleStart_PauseEmitsHostDiskLowOnceWithFreePercentAndPath(t *testing.T) {
	clock := &fakeClock{at: time.Unix(0, 0)}
	var events []DiskPauseEvent
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard:         pauseGuard("/vol/x", 2, 2, 2, 60),
		Wait:          clock.wait(time.Second),
		Now:           clock.now,
		MaxPause:      time.Hour,
		EmitDiskPause: func(e DiskPauseEvent) { events = append(events, e) },
	})
	if err != nil {
		t.Fatalf("RunCycleStart = %v, want nil once space returns", err)
	}
	if len(events) != 1 {
		t.Fatalf("emitted %d events over 3 paused checks, want exactly 1: %+v", len(events), events)
	}
	want := DiskPauseEvent{FreePct: 2, PausePct: 5, Path: "/vol/x", Outcome: DiskPauseOutcomePaused}
	if events[0] != want {
		t.Fatalf("event = %+v, want %+v", events[0], want)
	}
}

func TestRunCycleStart_HealthyDiskEmitsNothing(t *testing.T) {
	emitted := false
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard:         pauseGuard("/vol/x", 60),
		EmitDiskPause: func(DiskPauseEvent) { emitted = true },
	})
	if err != nil || emitted {
		t.Fatalf("healthy disk: err=%v emitted=%v, want nil and false", err, emitted)
	}
}

func TestRunCycleStart_PauseIsBoundedAndReturnsTheTypedError(t *testing.T) {
	clock := &fakeClock{at: time.Unix(0, 0)}
	var events []DiskPauseEvent
	waits := 0
	step := clock.wait(15 * time.Second)
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard:         pauseGuard("/vol/x", 2),
		Wait:          func(ctx context.Context) error { waits++; return step(ctx) },
		Now:           clock.now,
		MaxPause:      time.Minute,
		EmitDiskPause: func(e DiskPauseEvent) { events = append(events, e) },
	})
	if !errors.Is(err, ErrDiskPauseExceeded) {
		t.Fatalf("RunCycleStart = %v, want ErrDiskPauseExceeded", err)
	}
	if waits != 4 {
		t.Fatalf("waits = %d, want 4 (60s bound over 15s steps)", waits)
	}
	if len(events) != 2 || events[0].Outcome != DiskPauseOutcomePaused || events[1].Outcome != DiskPauseOutcomeGaveUp {
		t.Fatalf("events = %+v, want paused then gave_up", events)
	}
	if events[1].FreePct != 2 || events[1].Path != "/vol/x" {
		t.Fatalf("gave_up event = %+v, want free=2 path=/vol/x", events[1])
	}
}

func TestRunCycleStart_WaitDeadlineEndsThePauseWithTheTypedError(t *testing.T) {
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard:    pauseGuard("/vol/x", 2),
		Wait:     func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		MaxPause: 20 * time.Millisecond,
	})
	if !errors.Is(err, ErrDiskPauseExceeded) {
		t.Fatalf("RunCycleStart = %v, want ErrDiskPauseExceeded", err)
	}
}

func TestRunCycleStart_CancelledParentIsNotReportedAsTheBound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunCycleStart(ctx, CycleStartOptions{
		Guard:    pauseGuard("/vol/x", 2),
		MaxPause: time.Hour,
	})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrDiskPauseExceeded) {
		t.Fatalf("RunCycleStart = %v, want context.Canceled only", err)
	}
}

func TestRunCycleStart_ZeroMaxPauseTakesThePolicyDefault(t *testing.T) {
	clock := &fakeClock{at: time.Unix(0, 0)}
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard: pauseGuard("/vol/x", 2),
		Wait:  clock.wait(10 * time.Minute),
		Now:   clock.now,
	})
	if !errors.Is(err, ErrDiskPauseExceeded) {
		t.Fatalf("RunCycleStart = %v, want ErrDiskPauseExceeded at the 30 minute default", err)
	}
	if clock.at.Sub(time.Unix(0, 0)) != 30*time.Minute {
		t.Fatalf("gave up at %v, want 30m", clock.at.Sub(time.Unix(0, 0)))
	}
}

func TestRunCycleStart_UnreadableErrorFromWaitIsReturnedAsIs(t *testing.T) {
	boom := errors.New("wait broke")
	err := RunCycleStart(context.Background(), CycleStartOptions{
		Guard:    pauseGuard("/vol/x", 2),
		Wait:     func(context.Context) error { return boom },
		MaxPause: time.Hour,
	})
	if !errors.Is(err, boom) {
		t.Fatalf("RunCycleStart = %v, want the wait error", err)
	}
}
