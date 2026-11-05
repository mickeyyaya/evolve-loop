package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const runBudget = 5 * time.Second

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(runBudget)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestRun_FinishSendsTheWaveEndAndWaitsForTheRecord(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	run := r.d.Start(context.Background(), time.Hour)

	if err := run.Finish(runBudget); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if !rec.Ended || rec.Members[6].Status != MemberComposed {
		t.Fatalf("record = %+v, want ended with the member composed", rec)
	}
}

func TestRun_TimerTicksFlushWhileTheLanesRun(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	run := r.d.Start(context.Background(), 5*time.Millisecond)

	waitFor(t, "a timer tick to flush", func() bool { return len(r.store.get(5).Batches) == 1 })
	if err := run.Finish(runBudget); err != nil {
		t.Fatal(err)
	}
	if !r.store.get(5).Ended {
		t.Error("the wave did not end")
	}
}

func TestRun_AStalledTimerTickIsCutAtTheLimitAndFinishReturnsInTime(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	entered := make(chan struct{}, 1)
	r.actor.block = func(ctx context.Context) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
	}
	run := r.d.Start(context.Background(), 5*time.Millisecond)
	<-entered

	start := time.Now()
	run.Finish(50 * time.Millisecond)

	if elapsed := time.Since(start); elapsed > runBudget {
		t.Fatalf("Finish took %v, want it bounded by the limit", elapsed)
	}
	select {
	case <-run.done:
	default:
		t.Fatal("the goroutine outlived Finish")
	}
}

func TestRun_ACancelledLoopStopsATimerTickButNotTheWaveEndRequest(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	entered := make(chan struct{}, 1)
	var calls int
	r.actor.block = func(ctx context.Context) {
		calls++
		if calls == 1 {
			entered <- struct{}{}
			<-ctx.Done()
		}
	}
	parent, cancel := context.WithCancel(context.Background())
	run := r.d.Start(parent, 5*time.Millisecond)
	<-entered
	cancel()

	if err := run.Finish(runBudget); err != nil {
		t.Fatalf("Finish = %v, want the wave end to run on a context that the cancelled loop does not cancel", err)
	}
	if !r.store.get(5).Ended {
		t.Fatal("the cancelled loop kept the wave from ending")
	}
}

func TestRun_APanicBecomesADriverFailureAndFinishDoesNotHang(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Actors["shadow"] = &panicActor{}
	run := r.d.Start(context.Background(), time.Hour)

	run.Finish(runBudget)

	codes := r.sink.codes()
	if len(codes) != 1 || codes[0] != "SHIP_INTEGRATION_DRIVER_FAILED" {
		t.Fatalf("signals = %v", codes)
	}
	if e := r.sink.events[0]; e.Fields["step"] != "panic" || !strings.Contains(e.Fields["error"], "boom") {
		t.Errorf("event = %+v", e)
	}
}

type panicActor struct{ fakeActor }

func (*panicActor) Resume(context.Context, WaveRecord) (WaveRecord, error) { panic("boom") }

func TestRun_AStaleEpochEndsTheGoroutineAndFinishReturnsTheStaleError(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Epoch = &fixedEpoch{n: 99}
	run := r.d.Start(context.Background(), 5*time.Millisecond)
	waitFor(t, "the goroutine to end", func() bool {
		select {
		case <-run.done:
			return true
		default:
			return false
		}
	})
	if err := run.Finish(runBudget); err != nil && !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("Finish = %v", err)
	}
}

func TestRun_FinishOfNoRunIsANoOp(t *testing.T) {
	t.Parallel()
	var run *Run
	if err := run.Finish(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRun_FinishReturnsTheErrorOfTheWaveEndTick(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.actor.flushErr = errors.New("git broke")
	run := r.d.Start(context.Background(), time.Hour)
	if err := run.Finish(runBudget); err == nil || !strings.Contains(err.Error(), "git broke") {
		t.Fatalf("Finish = %v", err)
	}
}
