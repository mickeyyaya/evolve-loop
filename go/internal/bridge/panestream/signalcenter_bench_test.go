package panestream

// signalcenter_bench_test.go — cycle-433 slice S5, Task 2
// (s5-resolve-sharding-decision): measures concurrent Observe throughput on
// DISTINCT session keys — the metric that exposes whether sc.mu's
// write-lock-across-Assess() serializes independent sessions (ADR-0068 KF2/H1).
// Its measured result (see docs/architecture/adr/0068-*.md "Consequences")
// drives the resolution of the ADR's "Deferred (S5)" per-session-sharding
// decision.

import (
	"strconv"
	"sync/atomic"
	"testing"
)

// BenchmarkSignalCenter_ParallelObserve runs one goroutine per parallel
// worker, each repeatedly calling Observe on its OWN distinct session key —
// the ParallelEvaluate shape (N independent sessions, no key sharing). If the
// global RWMutex write-lock across Assess() serializes independent sessions,
// ns/op will stay flat (or worsen) as GOMAXPROCS/parallelism increases instead
// of improving; run with `-cpu=1,2,4,8` to compare.
func BenchmarkSignalCenter_ParallelObserve(b *testing.B) {
	sc := NewSignalCenter()
	profile := Profiles["claude"]
	var keySeq int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := "bench-sess-" + strconv.FormatInt(atomic.AddInt64(&keySeq, 1), 10)
		const content = "⏺ working on task\n❯ \n"
		for pb.Next() {
			sc.Observe(key, content, profile)
		}
	})
}
