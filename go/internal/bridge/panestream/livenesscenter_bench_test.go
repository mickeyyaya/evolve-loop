package panestream

import (
	"strconv"
	"sync/atomic"
	"testing"
)

func BenchmarkSignalCenter_ParallelObserve(b *testing.B) {
	sc := NewLivenessCenter()
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
