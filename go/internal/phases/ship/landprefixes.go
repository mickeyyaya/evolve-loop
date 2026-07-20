package ship

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// LandPrefixes is the LIVE composed main-push driver — the production caller that
// makes fleet.PrefixQueue.ResolveCulprit non-inert (the no-inert-API floor). Where
// PlanLanding only EMITS the prefix plan, LandPrefixes actually resolves culprits
// against a live verify gate and returns the landed / ejected lane IDs.
//
//   - "prefix-queue": enqueue the lanes into the single-writer PrefixQueue and
//     route through ResolveCulprit(verify). The composer — not this driver —
//     owns positional-NNFI culprit resolution and the post-ejection whole-set
//     re-verify (no poisoned composite lands). verify(landed) always holds.
//   - any other mode ("per-lane", the default; or an unknown value already
//     failed-safe to "per-lane" by policy.FleetConfig): each lane lands
//     independently, verified on its own, matching the legacy per-lane path.
//
// An empty lane set yields nil, nil in both modes.
func LandPrefixes(cfg policy.FleetConfig, lanes []fleet.LaneCandidate, verify func(laneIDs []string) bool) (landed, ejected []string) {
	if len(lanes) == 0 {
		return nil, nil
	}
	if cfg.Landing == "prefix-queue" {
		q := fleet.NewPrefixQueue()
		for _, l := range lanes {
			q.Enqueue(l)
		}
		return q.ResolveCulprit(verify)
	}
	// per-lane (legacy default): each lane stands or falls on its own.
	for _, l := range lanes {
		if verify([]string{l.ID}) {
			landed = append(landed, l.ID)
		} else {
			ejected = append(ejected, l.ID)
		}
	}
	return landed, ejected
}
