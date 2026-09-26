package loopwave

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// WidenNarrowDecision widens a prior triage decision to count file-disjoint lanes
// from the inbox backlog, re-marshalled as {"top_n": …} only; otherwise it returns data.
func WidenNarrowDecision(data []byte, evolveDir string, count int, protected func(string) bool) []byte {
	if count < 2 {
		return data
	}
	d, ok := parseDecision(data)
	// Floors dispatch ahead of top_n, and a top_n-only re-marshal would drop them.
	if !ok || len(d.CommittedFloors) > 0 {
		return data
	}
	committed, pruned := pruneCommitted(evolveDir, d)
	// A pruned id disarms both shortcuts: the original bytes still carry it.
	if !pruned && len(committed) >= count {
		return data
	}
	topN := widenAndDeepen(evolveDir, committed, count, protected)
	if !pruned && len(topN) <= len(committed) {
		return data
	}
	return remarshalTopN(data, topN)
}

// pruneCommitted drops consumed ids from the decision's top_n. It runs before
// the fleet-width shortcut, which would otherwise re-pin a consumed id.
func pruneCommitted(evolveDir string, d decision) (committed []triagecap.FleetCandidate, pruned bool) {
	committed = make([]triagecap.FleetCandidate, 0, len(d.TopN))
	for _, c := range d.TopN {
		if c.ID != "" {
			committed = append(committed, triagecap.FleetCandidate{ID: c.ID, Files: c.Files})
		}
	}
	if kept := triagecap.PruneConsumed(evolveDir, committed); len(kept) < len(committed) {
		return kept, true
	}
	return committed, false
}

// widenAndDeepen backfills committed to count disjoint lanes and deepens each
// lane with its same-file cluster mates, as the inbox seed does.
func widenAndDeepen(evolveDir string, committed []triagecap.FleetCandidate, count int, protected func(string) bool) []map[string]any {
	backlog := triagecap.ReadInboxBacklog(evolveDir, protected)
	widened := triagecap.WidenTopNToFleetWidth(committed, backlog, count)
	return cards(triagecap.ExpandWithClusterMates(widened, backlog, inboxbatch.DefaultMaxItems))
}
