package fixtures

import (
	"sync"
	"testing"
)

// StressN runs a concurrency stress fixture: it launches n goroutines, each
// invoking fn(g, i) for i in [0, k), released together by a closed-channel
// barrier so they are not serialized by staggered startup, then blocks until
// every goroutine finishes. fn receives its goroutine index g in [0, n) and
// iteration index i. (The barrier minimizes staggering; it is not a hard
// real-time guarantee of simultaneous execution under low GOMAXPROCS.)
//
// fn must not call t.Fatal/t.FailNow from inside the goroutine — those call
// runtime.Goexit on only that goroutine; assert with t.Error, or return/record
// a value and check it after StressN returns.
//
// This is the canonical stress harness for the campaign's concurrency-test
// backfill (Phase 2): pair it with an invariant assertion (chain intact, no
// overspend, no torn lines) under `-race`, never a bare "didn't panic". See
// docs/testing.md for the pattern and the behavior-named test-name shape.
func StressN(t testing.TB, n, k int, fn func(g, i int)) {
	t.Helper()
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for g := 0; g < n; g++ {
		go func(g int) {
			defer wg.Done()
			<-start // barrier — block until all goroutines are spawned
			for i := 0; i < k; i++ {
				fn(g, i)
			}
		}(g)
	}
	close(start) // release every goroutine at once
	wg.Wait()
}
