package usageprobe

import (
	"context"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

type QuotaReader struct {
	Probe func(ctx context.Context, family string) (string, error)
	Read  func(family, pane string, now time.Time) []quotastate.UsageWindow
	Now   time.Time
}

func ProbeQuota(ctx context.Context, families []string, r QuotaReader) []quotastate.QuotaState {
	out := make([]quotastate.QuotaState, 0, len(families))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, family := range families {
		family := family
		wg.Add(1)
		go func() {
			defer wg.Done()
			pane, err := r.Probe(ctx, family)
			if err != nil {
				return
			}
			states := quotastate.StatesOf(r.Read(family, pane, r.Now), r.Now)
			mu.Lock()
			out = append(out, states...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}
