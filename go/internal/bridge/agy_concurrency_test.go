package bridge

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// A frozen clock gives every lane the same timestamp, so only the atomic nonce can separate the names.
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
			// Same run, cycle and agent for every lane: a fleet dispatching one phase across lanes in one second.
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
