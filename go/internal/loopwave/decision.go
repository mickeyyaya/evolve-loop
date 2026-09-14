package loopwave

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// WidenNarrowDecision turns a present-but-narrow prior triage-decision.json
// into a fleet-width one: the committed top_n (pruned of consumed ids through
// triagecap.PruneConsumed) is backfilled from the inbox backlog up to `count`
// mutually file-disjoint lanes and each lane deepened with its cluster mates,
// then re-marshalled as {"top_n": …} only (remarshalTopN — Q-W3). Best-effort:
// count<2, an unparseable decision, or one carrying committed_floors (which
// PlanFromTriage dispatches ahead of top_n — a top_n-only re-marshal would
// silently DROP the floors) returns the original bytes; so does a decision
// already fleet-width or one nothing can be added to — UNLESS the prune
// dropped an id, which disarms BOTH shortcuts (those bytes still carry the
// consumed id; cycle-1116 — Q-W2). An EMPTY top_n is not a no-op: it widens
// fully from the backlog (cycle-554).
func WidenNarrowDecision(data []byte, evolveDir string, count int, protected func(string) bool) []byte {
	if count < 2 {
		return data
	}
	d, ok := parseDecision(data)
	if !ok || len(d.CommittedFloors) > 0 {
		return data
	}
	committed, pruned := pruneCommitted(evolveDir, d)
	if !pruned && len(committed) >= count {
		return data // already fleet-width — committed intent is authoritative
	}
	topN := widenAndDeepen(evolveDir, committed, count, protected)
	if !pruned && len(topN) <= len(committed) {
		return data // nothing disjoint to add, no mates to deepen with
	}
	return remarshalTopN(data, topN)
}

// pruneCommitted builds the committed candidates from the decision's top_n
// (empty ids skipped) and drops the ones the inbox lifecycle has already
// consumed — the SAME primitive as the fresh-seed path. This runs BEFORE the
// fleet-width short-circuit: a decision already `count` wide would otherwise
// re-pin a consumed id into the next wave's lane-scope.json.
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

// widenAndDeepen backfills committed from the backlog to `count` disjoint
// lanes (WidenTopNToFleetWidth) and deepens each with its pending same-file
// cluster mates (ExpandWithClusterMates) — the mid-batch primary path
// amortizes a lane's worktree/build/audit across its cluster exactly like the
// fresh inbox seed does. The cap mirrors the seed call site (the compiled
// inboxbatch.DefaultMaxItems).
func widenAndDeepen(evolveDir string, committed []triagecap.FleetCandidate, count int, protected func(string) bool) []map[string]any {
	backlog := triagecap.ReadInboxBacklog(evolveDir, protected)
	widened := triagecap.WidenTopNToFleetWidth(committed, backlog, count)
	return cards(triagecap.ExpandWithClusterMates(widened, backlog, inboxbatch.DefaultMaxItems))
}
