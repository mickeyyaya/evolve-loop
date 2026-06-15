package fixtures

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStressN_RunsAllGoroutinesSimultaneously(t *testing.T) {
	const n, k = 8, 5
	var calls, running, peak atomic.Int64

	// Rendezvous: each goroutine signals arrival on its first call and blocks
	// until ALL n have arrived. If StressN released them simultaneously, all n
	// are in fn at once and peak concurrency reaches n. If StressN serialized
	// them, the first goroutine blocks forever and the rendezvous times out —
	// proving the barrier works, not merely that nothing crashed.
	var arrived sync.WaitGroup
	arrived.Add(n)
	released := make(chan struct{})
	go func() { arrived.Wait(); close(released) }()

	first := make([]atomic.Bool, n)
	StressN(t, n, k, func(g, i int) {
		calls.Add(1)
		cur := running.Add(1)
		for { // record peak concurrency
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		if first[g].CompareAndSwap(false, true) {
			arrived.Done()
			select {
			case <-released:
			case <-time.After(2 * time.Second):
				t.Errorf("goroutine %d never reached the rendezvous — StressN serialized them", g)
			}
		}
		running.Add(-1)
	})

	if got := calls.Load(); got != int64(n*k) {
		t.Errorf("fn invoked %d times, want n*k = %d", got, n*k)
	}
	if got := peak.Load(); got < int64(n) {
		t.Errorf("peak concurrency = %d, want >= %d (goroutines were not released simultaneously)", got, n)
	}
}

func TestStressN_ZeroGoroutinesIsNoop(t *testing.T) {
	var calls atomic.Int64
	StressN(t, 0, 100, func(g, i int) { calls.Add(1) })
	if calls.Load() != 0 {
		t.Errorf("n=0 must invoke fn 0 times, got %d", calls.Load())
	}
}
