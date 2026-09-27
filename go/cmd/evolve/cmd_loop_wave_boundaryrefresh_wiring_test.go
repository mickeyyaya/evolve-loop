package main

// acs-predicate: config-check — caller-existence is an inherent source-
// structure check; maybeRefreshChainBoundary's behavior is already pinned by
// cmd_loop_chain_boundaryrefresh_test.go / _hardening_test.go (both GREEN).
import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary(t *testing.T) {
	n, err := acsassert.CountInGoFunc("cmd_loop_window.go", "prepareIteration", "maybeRefreshChainBoundary")
	if err != nil {
		t.Fatalf("CountInGoFunc(prepareIteration, maybeRefreshChainBoundary): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopBatch does not call maybeRefreshChainBoundary (count=%d); the wave/fleet batch loop can still run for hours on a stale binary after a chain-less `evolve loop --max-cycles N` — only runLoopChain gets the cycle-1314 self-heal today", n)
	}
	wired, err := acsassert.CountInGoFunc("cmd_loop_batch.go", "run", "prepareIteration")
	if err != nil {
		t.Fatalf("CountInGoFunc(loopBatchCoordinator.run, prepareIteration): %v", err)
	}
	if wired != 1 {
		t.Errorf("loopBatchCoordinator.run calls prepareIteration %d times, want exactly once at each iteration boundary", wired)
	}
}

func TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers(t *testing.T) {
	for _, fn := range []string{"dispatchIteration", "forceOneLaneDispatch", "minWidthRepair"} {
		n, err := acsassert.CountInGoFunc("cmd_loop_wave.go", fn, "maybeRefreshChainBoundary")
		if err != nil {
			t.Fatalf("CountInGoFunc(%s, maybeRefreshChainBoundary): %v", fn, err)
		}
		if n > 0 {
			t.Errorf("%s calls maybeRefreshChainBoundary (count=%d) — the refresh belongs at runLoop's own boundary point, not inside a per-lane dispatch helper (would re-check staleness mid-lane)", fn, n)
		}
	}
}
