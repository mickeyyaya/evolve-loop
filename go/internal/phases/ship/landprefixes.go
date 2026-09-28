package ship

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// LandPrefixes lands lanes per cfg.Landing, resolving culprits through verify
// and returning the landed and ejected lane IDs.
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
	for _, l := range lanes {
		if verify([]string{l.ID}) {
			landed = append(landed, l.ID)
		} else {
			ejected = append(ejected, l.ID)
		}
	}
	return landed, ejected
}
