package ship

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// PlanLanding is the single wiring seam the fleet main-push path consults to
// decide how PASS lanes land. It routes on the resolved policy landing mode:
//
//   - "per-lane" (the default, byte-identical to today): every lane lands
//     independently — one singleton group per lane, never a multi-lane group.
//   - "prefix-queue": the lanes are enqueued into the single-writer
//     fleet.PrefixQueue and the returned plan IS its ComposePrefixes() output,
//     so the composer — not this seam — owns the main-push decision.
//
// Any non-"prefix-queue" mode (including an unknown value already failed-safe to
// "per-lane" by policy.FleetConfig) takes the legacy per-lane path. An empty
// lane set yields an empty plan in both modes.
func PlanLanding(cfg policy.FleetConfig, lanes []fleet.LaneCandidate) [][]string {
	if len(lanes) == 0 {
		return nil
	}
	if cfg.Landing == "prefix-queue" {
		q := fleet.NewPrefixQueue()
		for _, l := range lanes {
			q.Enqueue(l)
		}
		return q.ComposePrefixes()
	}
	// per-lane (legacy default): each lane lands on its own.
	plan := make([][]string, 0, len(lanes))
	for _, l := range lanes {
		plan = append(plan, []string{l.ID})
	}
	return plan
}
