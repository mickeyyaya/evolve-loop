package fleet

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func peakLiveF4(t *testing.T, cfg PoolConfig, lanes int, wantAtLeast int) int {
	t.Helper()
	backlog := make([]Todo, lanes)
	for i := range backlog {
		backlog[i] = Todo{ID: fmt.Sprintf("t%d", i), Files: []string{fmt.Sprintf("f%d.go", i)}}
	}
	var mu sync.Mutex
	live, peak := 0, 0
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		mu.Lock()
		live++
		if live > peak {
			peak = live
		}
		mu.Unlock()
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			mu.Lock()
			reached := live >= wantAtLeast
			mu.Unlock()
			if reached {
				break
			}
			time.Sleep(time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		live--
		mu.Unlock()
		return 0, nil
	}

	results := RunPool(context.Background(), cfg, backlog, launch, nil)

	if len(results) != lanes {
		t.Fatalf("results = %d, want %d", len(results), lanes)
	}
	for _, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			t.Fatalf("result = %+v", r)
		}
	}
	return peak
}

func TestRunPool_ConcurrencyBelowTheTargetCapsTheLiveLanes(t *testing.T) {
	if got := peakLiveF4(t, PoolConfig{Target: 4, Concurrency: 2}, 5, 2); got != 2 {
		t.Fatalf("peak live lanes = %d, want the concurrency cap 2", got)
	}
}

func TestRunPool_ConcurrencyAboveOrAtTheTargetDoesNotRaiseTheTarget(t *testing.T) {
	if got := peakLiveF4(t, PoolConfig{Target: 2, Concurrency: 5}, 4, 2); got != 2 {
		t.Fatalf("peak live lanes = %d, want the target 2", got)
	}
	if got := peakLiveF4(t, PoolConfig{Target: 3}, 5, 3); got != 3 {
		t.Fatalf("with no concurrency cap the target rules: peak = %d", got)
	}
}

func TestRunPool_ATargetBelowOneStillRunsOneLaneAtATime(t *testing.T) {
	for _, cfg := range []PoolConfig{{Target: 0}, {Target: -3}, {Target: 0, Concurrency: 4}} {
		if got := peakLiveF4(t, cfg, 3, 1); got != 1 {
			t.Errorf("cfg %+v: peak live lanes = %d, want 1 (a pool that runs nothing would hang the wave)", cfg, got)
		}
	}
}
