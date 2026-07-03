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

// ProbeQuota captures each family's /usage pane through the injected probe and
// parses it into a quotastate.QuotaState. It runs the families concurrently and
// is fail-open: a family is OMITTED unless it yields a PROBED state (real
// numbers parsed). That covers both an errored probe (unsupported usage command
// like ollama's, or a bridge/timeout failure) AND a successful-but-unparseable
// pane (Parse → Source=unknown) — neither produces a fabricated cap, so the
// allocator treats an absent family as no signal (floor fallback). now is
// injected so reset-time parsing is deterministic. The returned slice order is
// not significant; an empty families list yields an empty (non-nil) slice.
func ProbeQuota(ctx context.Context, families []string, probe func(ctx context.Context, family string) (string, error), now time.Time) []quotastate.QuotaState {
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
			pane, err := probe(ctx, family)
			if err != nil {
				return // unsupported or errored probe → omit (fail-open)
			}
			q := quotastate.Parse(family, pane, now)
			if q.Source != quotastate.SourceProbed {
				return // responded, but no numbers parsed → omit (no fabricated state)
			}
			mu.Lock()
			out = append(out, q)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}
