//go:build unix

package fleet

import (
	"context"
	"syscall"
	"testing"
	"time"
)

const (
	cancelSpinWindow = 300 * time.Millisecond
	cancelSpinBudget = 150 * time.Millisecond
)

func processCPU(t *testing.T) time.Duration {
	t.Helper()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatal(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func TestPool_ACancelWithTwoRunningLanesReturnsAfterBothCompleteWithoutSpinning(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		started <- struct{}{}
		<-release
		return 0, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(ctx, PoolConfig{Target: 2, Hold: NewFloorHold()}, []Todo{{ID: "A", Files: []string{"a.go"}}, {ID: "B", Files: []string{"b.go"}}}, launch, nil)
	}()
	<-started
	<-started
	cancel()
	before := processCPU(t)
	select {
	case <-done:
		t.Fatal("the pool returned with two lanes still running")
	case <-time.After(cancelSpinWindow):
	}
	if spent := processCPU(t) - before; spent > cancelSpinBudget {
		t.Fatalf("a cancel with running lanes burned %v of CPU in %v: the done case spins", spent, cancelSpinWindow)
	}
	close(release)
	select {
	case res := <-done:
		if len(res) != 2 || res[0].Err != nil || res[1].Err != nil {
			t.Fatalf("results = %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the pool did not return after both lanes completed")
	}
}
