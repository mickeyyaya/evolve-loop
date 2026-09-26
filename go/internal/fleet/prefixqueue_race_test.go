package fleet

import (
	"fmt"
	"sync"
	"testing"
)

func TestPrefixQueue_ConcurrentEnqueue_NoLostWork(t *testing.T) {
	const n = 500
	q := NewPrefixQueue()
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			q.Enqueue(LaneCandidate{
				ID:    fmt.Sprintf("L%d", i),
				Tier:  TierMaybe,
				Files: []string{fmt.Sprintf("go/internal/f%d/f.go", i)},
			})
			// Interleave AIMD control writes to race the window field too.
			if i%2 == 0 {
				q.OnGreen()
			} else {
				q.OnRed()
			}
			_ = q.Window()
		}(i)
	}
	wg.Wait()

	// Disjoint files keep every lane in one group, so ComposePrefixes yields one prefix per lane.
	if got := len(q.ComposePrefixes()); got != n {
		t.Fatalf("%d lanes survived %d concurrent Enqueues — lost appends under contention", got, n)
	}
	if got := q.Window(); got < 1 {
		t.Fatalf("AIMD window = %d after concurrent reds — floor of 1 breached", got)
	}
}
