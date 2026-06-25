package phaseblock

import (
	"sync"
	"testing"
)

// The phaseblock primitives (Compute, Verify, combine) are pure — no shared
// mutable state — so they MUST be safe to call concurrently: a digest is
// recorded at the post-phase chokepoint from whatever goroutine ran the phase,
// and Verify runs at ship. This test drives both from many goroutines over a
// shared read-only chain; run with -race to prove thread-safety.
func TestCompute_And_Verify_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	srcs := []fakeSource{
		{bin: "binA", commit: "cA", prof: "p1"},
		{bin: "binA", commit: "cA", prof: "p2", report: "r2", tree: "t2"},
	}
	// Establish the golden chain + Combined sequence serially.
	golden := buildChainNoT([]string{"scout", "build"}, srcs)

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			// Concurrent Compute must be deterministic: same inputs → same Combined.
			d0, err := Compute("scout", "run", "ts", "", srcs[0])
			if err != nil {
				t.Errorf("concurrent Compute error: %v", err)
				return
			}
			if d0.Combined != golden[0].Combined {
				t.Errorf("concurrent Compute non-deterministic: %q != %q", d0.Combined, golden[0].Combined)
				return
			}
			// Concurrent Verify over a shared read-only chain must be stable.
			if err := Verify(golden, "binA", "cA", allOK); err != nil {
				t.Errorf("concurrent Verify: %v", err)
			}
		}()
	}
	wg.Wait()
}

// buildChainNoT mirrors buildChain but takes no *testing.T, so it is safe to
// call once before fanning out goroutines (t.Helper/t.Fatal must stay on the
// test goroutine). Panics on the impossible misuse (Compute never errors for
// a fakeSource without injected errors).
func buildChainNoT(phases []string, srcs []fakeSource) []Digest {
	var chain []Digest
	prev := ""
	for i, ph := range phases {
		d, err := Compute(ph, "run", "ts", prev, srcs[i])
		if err != nil {
			panic(err)
		}
		chain = append(chain, d)
		prev = d.Combined
	}
	return chain
}
