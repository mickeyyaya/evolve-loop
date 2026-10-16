package phaseblock

import (
	"sync"
	"testing"
)

func TestCompute_And_Verify_ConcurrentSafe(t *testing.T) {
	t.Parallel()

	srcs := []fakeSource{
		{bin: "binA", commit: "cA", prof: "p1"},
		{bin: "binA", commit: "cA", prof: "p2", report: "r2", tree: "t2"},
	}
	golden := buildChainNoT([]string{"scout", "build"}, srcs)

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			d0, err := Compute("scout", "run", "ts", "", srcs[0])
			if err != nil {
				t.Errorf("concurrent Compute error: %v", err)
				return
			}
			if d0.Combined != golden[0].Combined {
				t.Errorf("concurrent Compute non-deterministic: %q != %q", d0.Combined, golden[0].Combined)
				return
			}
			if err := Verify(golden, "binA", "cA", allOK); err != nil {
				t.Errorf("concurrent Verify: %v", err)
			}
		}()
	}
	wg.Wait()
}

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
