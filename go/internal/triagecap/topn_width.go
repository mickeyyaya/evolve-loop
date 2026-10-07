package triagecap

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

type FleetCandidate struct {
	ID     string
	Weight float64
	Files  []string
}

func SelectFleetWidthTopN(ranked []FleetCandidate, count int) []FleetCandidate {
	if len(ranked) == 0 {
		return nil
	}
	if count < 2 {
		return []FleetCandidate{ranked[0]}
	}

	todos := make([]fleet.Todo, len(ranked))
	byID := make(map[string]FleetCandidate, len(ranked))
	for i, c := range ranked {
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

	for _, c := range backlog {
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
