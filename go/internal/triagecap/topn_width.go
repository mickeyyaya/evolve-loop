package triagecap

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// topn_width.go — fleet-width-aware, file-disjoint top_n selection (inbox
// triage-supply-disjoint-topn-for-fleet-width). Cycle-503 triage committed
// exactly 1 top_n task and starved the fleet wave planner of the >=2 disjoint
// tasks it needs to fan out 2 concurrent lanes. SelectFleetWidthTopN is the
// SSOT: it greedily packs the highest-weight candidates into up to `count`
// mutually FILE-DISJOINT lanes and returns one representative per non-empty
// lane, so the returned set is always safe to fan out 1:1 into concurrent
// `evolve cycle run` lanes without a cross-lane file collision.

// FleetCandidate is one triage backlog item eligible for top_n: its id, its
// selection weight (higher = preferred), and the repo files its cycle would
// touch. Files drive cross-lane disjointness — two candidates sharing a file
// cannot run as concurrent lanes without colliding on the shared tree.
type FleetCandidate struct {
	ID     string
	Weight float64
	Files  []string
}

// SelectFleetWidthTopN returns up to `count` mutually file-disjoint top_n
// representatives, highest-weight first. It delegates the disjoint packing to
// fleet.Partition (the SSOT greedy file-ownership algorithm) rather than
// duplicating it, then lifts one representative — the highest-weight member —
// out of each non-empty bucket.
//
// count<2 reproduces the legacy single-focus behavior byte-identically: exactly
// the single highest-weight candidate, independent of file overlap. When the
// backlog cannot fill `count` disjoint lanes, the widest disjoint set (>=1) is
// returned — never a fabricated/overlapping pairing.
func SelectFleetWidthTopN(candidates []FleetCandidate, count int) []FleetCandidate {
	if len(candidates) == 0 {
		return nil
	}
	// Highest weight first; stable so equal-weight ties preserve input order.
	sorted := make([]FleetCandidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Weight > sorted[j].Weight })

	if count < 2 {
		return []FleetCandidate{sorted[0]}
	}

	todos := make([]fleet.Todo, len(sorted))
	byID := make(map[string]FleetCandidate, len(sorted))
	for i, c := range sorted {
		todos[i] = fleet.Todo{ID: c.ID, Files: c.Files}
		byID[c.ID] = c
	}
	buckets, _ := fleet.Partition(todos, count)

	var out []FleetCandidate
	for _, b := range buckets {
		if len(b) == 0 {
			continue
		}
		// Partition preserves input order and the input was weight-sorted, so
		// b[0] is the highest-weight member of the bucket: the lane's rep.
		out = append(out, byID[b[0].ID])
	}
	return out
}
