package loopwave

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func WidenNarrowDecision(data []byte, evolveDir string, count int, protected func(string) bool) []byte {
	return widenNarrowDecision(data, evolveDir, count, func() []triagecap.FleetCandidate {
		return triagecap.ReadInboxBacklog(evolveDir, protected)
	})
}

func widenNarrowDecision(data []byte, evolveDir string, count int, backlog func() []triagecap.FleetCandidate) []byte {
	if count < 2 {
		return data
	}
	d, ok := parseDecision(data)
	if !ok || len(d.CommittedFloors) > 0 {
		return data
	}
	committed, pruned := pruneCommitted(evolveDir, d)
	if !pruned && len(committed) >= count {
		return data
	}
	topN := widenAndDeepen(evolveDir, committed, count, backlog)
	if !pruned && len(topN) <= len(committed) {
		return data
	}
	return remarshalTopN(data, topN)
}

func pruneCommitted(evolveDir string, d decision) (committed []triagecap.FleetCandidate, pruned bool) {
	committed = make([]triagecap.FleetCandidate, 0, len(d.TopN))
	for _, c := range d.TopN {
		if c.ID != "" {
			committed = append(committed, triagecap.FleetCandidate{ID: c.ID, Files: c.Files})
		}
	}
	if kept := triagecap.PruneUndispatchable(evolveDir, committed); len(kept) < len(committed) {
		return kept, true
	}
	return committed, false
}

func widenAndDeepen(evolveDir string, committed []triagecap.FleetCandidate, count int, readBacklog func() []triagecap.FleetCandidate) []map[string]any {
	backlog := readBacklog()
	widened := triagecap.WidenTopNToFleetWidth(committed, backlog, count)
	return cards(triagecap.ExpandWithClusterMates(widened, backlog, inboxbatch.DefaultMaxItems))
}
