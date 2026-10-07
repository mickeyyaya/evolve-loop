package usageprobe

// quotaprobe.go adds the MEASUREMENT path over the same probe seam the boolean
// Prober uses. Where Prober captures each family's /usage pane and throws the
// numbers away after a capped/not classification, ProbeQuota captures the same
// pane and PARSES it into a quotastate.QuotaState the fleetbudget allocator can
// size a wave against. Sharing the probe seam keeps a single way to reach a
// family's usage command (no second bridge-assembly path).

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
				return // unsupported or errored probe → omit (fail-open)
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
