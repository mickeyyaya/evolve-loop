package triagecap

import (
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// FleetCandidate is a backlog item eligible for top_n; candidates sharing a file cannot run as concurrent lanes.
type FleetCandidate struct {
	ID     string
	Weight float64
	Files  []string
	// Declared is inboxbatch.Item.DeclaredSurface from the backlog read; candidates built from a decision stay false.
	Declared bool
}

// SelectFleetWidthTopN returns one representative per mutually file-disjoint lane, up to count, via fleet.Partition.
// count<2 returns the single top-ranked candidate regardless of overlap.
func SelectFleetWidthTopN(candidates []FleetCandidate, count int) []FleetCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sorted := rankForDispatch(candidates)

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
		// Partition keeps input order, so b[0] is the bucket's top-ranked member.
		out = append(out, byID[b[0].ID])
	}
	return out
}

// WidenTopNToFleetWidth keeps every committed candidate verbatim, even overlapping ones, and backfills
// top-ranked backlog candidates that touch no claimed file, up to count. count<2 returns committed unchanged.
func WidenTopNToFleetWidth(committed, backlog []FleetCandidate, count int) []FleetCandidate {
	if count < 2 {
		return committed
	}
	out := make([]FleetCandidate, len(committed))
	copy(out, committed)

	claimed := map[string]bool{} // normalized file -> already owned by the selection
	seenID := make(map[string]bool, len(committed))
	for _, c := range committed {
		seenID[c.ID] = true
		for _, f := range c.Files {
			claimed[filepath.Clean(f)] = true
		}
	}
	if len(out) >= count {
		return out
	}

	sorted := rankForDispatch(backlog)

	for _, c := range sorted {
		if len(out) >= count {
			break
		}
		if seenID[c.ID] || overlapsClaimed(c.Files, claimed) {
			continue
		}
		out = append(out, c)
		seenID[c.ID] = true
		for _, f := range c.Files {
			claimed[filepath.Clean(f)] = true
		}
	}
	return out
}

// overlapsClaimed cleans paths as fleet.Partition does; Partition itself would drop overlapping committed items.
func overlapsClaimed(files []string, claimed map[string]bool) bool {
	for _, f := range files {
		if claimed[filepath.Clean(f)] {
			return true
		}
	}
	return false
}

// rankForDispatch is the one ordering every seed path shares: weight, then a verified declared surface,
// then input order. The weight stays the priority; admissibility only breaks ties.
// See ADR-0074.
func rankForDispatch(cands []FleetCandidate) []FleetCandidate {
	sorted := make([]FleetCandidate, len(cands))
	copy(sorted, cands)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Weight != sorted[j].Weight {
			return sorted[i].Weight > sorted[j].Weight
		}
		return sorted[i].Declared && !sorted[j].Declared
	})
	return sorted
}
