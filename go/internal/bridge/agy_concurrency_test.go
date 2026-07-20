// agy_concurrency_test.go — the agy-only fleet concurrency contract. In an
// agy-ONLY environment every lane, every phase, every retry launches an
// agy-tmux pane; the ONE deterministic invariant that keeps concurrent lanes
// from fighting over a single tmux session is resolveSession's atomic
// per-process nonce (ADR-0049 N15). The pre-existing nonce test only mints two
// sessions SEQUENTIALLY; these tests mint many CONCURRENTLY under a frozen
// clock (worst case: every lane in the same wall-clock second) and assert with
// `-race` that the atomic increment is safe and every name is distinct.
package bridge

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestResolveSession_AgyConcurrent_UniqueUnderRace — N concurrent agy launches
// under an IDENTICAL frozen clock must every one get a distinct, tmux-safe,
// agy-prefixed session name. Frozen clock is the adversary: the second-
// granularity timestamp is identical for all N, so only the atomic nonce can
// separate them. Each goroutine writes its own slice index (no test-induced
// race), so `-race` exercises resolveSession's OWN concurrency safety —
// specifically the shared ephemeralSessionNonce.Add across goroutines.
func TestResolveSession_AgyConcurrent_UniqueUnderRace(t *testing.T) {
	frozen := time.Unix(1_700_000_000, 0)
	deps := Deps{Now: func() time.Time { return frozen }}.withDefaults()

	const lanes = 64
	names := make([]string, lanes)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < lanes; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all goroutines at once to maximize contention
			// Same run/cycle/agent for every lane — the realistic worst case
			// (a fleet dispatching the same phase across lanes in one second).
			cfg := &Config{Cycle: 100, Agent: "audit", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
			names[i], _ = resolveSession(cfg, deps, "evolve-bridge-agy-")
		}(i)
	}
	close(start)
	wg.Wait()

	seen := make(map[string]struct{}, lanes)
	for _, n := range names {
		if _, dup := seen[n]; dup {
			t.Fatalf("concurrent agy launches collided on session name %q — two agy panes would fight over one tmux session (agy-only fleet corruption)", n)
		}
		seen[n] = struct{}{}
		if !strings.HasPrefix(n, "evolve-bridge-agy-") {
			t.Errorf("agy session lost its driver prefix: %q", n)
		}
		if len(n) > 64 {
			t.Errorf("agy session exceeds tmux's 64-char ceiling: %q (%d)", n, len(n))
		}
	}
	if len(seen) != lanes {
		t.Fatalf("got %d unique agy session names, want %d", len(seen), lanes)
	}
}

// TestResolveSession_AgyConcurrent_MixedRunsAndCycles — the multi-run/multi-cycle
// worst case: concurrent lanes across DIFFERENT runs and cycles must still be
// unique AND each must carry its own run-scope token (CB.5), so `tmux ls` and
// the observer watchers can attribute every agy pane to the right run under a
// live agy-only fleet.
func TestResolveSession_AgyConcurrent_MixedRunsAndCycles(t *testing.T) {
	frozen := time.Unix(1_700_000_000, 0)
	deps := Deps{Now: func() time.Time { return frozen }}.withDefaults()

	runs := []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01BX5ZZKBKACTAV9WEVGEMMVRZ"}
	const perRun = 24
	type res struct{ run, name string }
	out := make([]res, len(runs)*perRun)
	var wg sync.WaitGroup
	for r, run := range runs {
		for c := 0; c < perRun; c++ {
			idx := r*perRun + c
			wg.Add(1)
			go func(idx, cycle int, run string) {
				defer wg.Done()
				cfg := &Config{Cycle: cycle, Agent: "build", RunID: run}
				n, _ := resolveSession(cfg, deps, "evolve-bridge-agy-")
				out[idx] = res{run: run, name: n}
			}(idx, c, run)
		}
	}
	wg.Wait()

	seen := make(map[string]struct{}, len(out))
	for _, r := range out {
		if _, dup := seen[r.name]; dup {
			t.Fatalf("cross-run/cycle agy session collision: %q", r.name)
		}
		seen[r.name] = struct{}{}
		// run-scope token = first 8 chars of the ULID after the "r" marker.
		wantTok := "r" + r.run[:8]
		if !strings.Contains(r.name, wantTok) {
			t.Errorf("agy session %q missing run-scope token %q (unattributable pane)", r.name, wantTok)
		}
	}
}
