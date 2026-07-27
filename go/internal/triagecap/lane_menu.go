package triagecap

// lane_menu.go — fleet lanes consume a MENU, not a single todo
// (fleet-lane-batch-menu). SelectFleetWidthTopN hands each lane one
// representative and discards its partition bucket-mates, so a fleet cycle
// could never amortize its worktree/build/audit across the same-file items
// the batching layer deliberately groups (live proof: batch-14 wave-1, both
// lane scopes single-element while the triage prompt's "prefer a whole batch
// as top_n" guidance sat unreachable). Expansion deepens each lane with its
// same-file cluster mates; triage inside the lane keeps full authority to
// commit a subset and leave the rest pending — unworked menu ids are never
// claimed, so they simply remain dispatchable backlog.

import (
	"path/filepath"
	"sort"
)

// ExpandWithClusterMates deepens a selection (one lane per member) into lane
// menus of up to perLane candidates each. A backlog candidate joins the ONE
// lane whose claimed files it overlaps — and then claims its own files for
// that lane, so a cluster grows transitively without ever crossing lanes. A
// candidate overlapping zero lanes is WIDTH material (its own future lane),
// never depth-filler; one overlapping two lanes would bridge two concurrent
// worktrees into a ship-time collision (the same rule as fleet.Partition's
// deferred case) — it joins neither. perLane < 2 returns the selection as
// single-item menus. Deterministic: backlog is walked highest-weight first
// (stable), input order breaking ties.
//
// The selection is normally mutually file-disjoint, but a WidenTopNToFleetWidth
// committed prefix is explicitly allowed to overlap itself (committed intent is
// authoritative) — the FIRST lane touching a file keeps it, so mates attach to
// that lane and never to a later overlapping one; the overlapping reps
// themselves are re-merged into one lane downstream by fleet.Partition.
func ExpandWithClusterMates(selection, backlog []FleetCandidate, perLane int) [][]FleetCandidate {
	menus := make([][]FleetCandidate, 0, len(selection))
	owner := map[string]int{} // normalized file → owning lane index
	inMenu := make(map[string]bool, len(selection))
	for lane, rep := range selection {
		menus = append(menus, []FleetCandidate{rep})
		inMenu[rep.ID] = true
		for _, f := range rep.Files {
			cf := filepath.Clean(f)
			if _, taken := owner[cf]; !taken { // first lane keeps the file — no steal by a later overlapping rep
				owner[cf] = lane
			}
		}
	}
	if perLane < 2 {
		return menus
	}
	sorted := make([]FleetCandidate, len(backlog))
	copy(sorted, backlog)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Weight > sorted[j].Weight })
	for _, c := range sorted {
		if inMenu[c.ID] {
			continue
		}
		lane, ok := soleOwningLane(owner, c.Files)
		if !ok || len(menus[lane]) >= perLane {
			continue
		}
		menus[lane] = append(menus[lane], c)
		inMenu[c.ID] = true
		for _, f := range c.Files {
			owner[filepath.Clean(f)] = lane
		}
	}
	return menus
}

// soleOwningLane resolves which single lane owns any of files. ok is false for
// zero owners (independent work) and for two or more (a bridge).
func soleOwningLane(owner map[string]int, files []string) (lane int, ok bool) {
	lane = -1
	for _, f := range files {
		l, claimed := owner[filepath.Clean(f)]
		if !claimed {
			continue
		}
		if lane != -1 && lane != l {
			return 0, false // bridges two lanes
		}
		lane = l
	}
	return lane, lane != -1
}

// SelectWaveSeedMenus is the menu-aware wave seed: the inbox backlog packed
// into up to `count` mutually file-disjoint lanes (SelectFleetWidthTopN, the
// existing SSOT) with each lane deepened to at most perLane same-file cluster
// mates. The outer slice length is the realizable WAVE WIDTH — callers gate
// on it exactly as they gated on len(SelectWaveSeedTopN(...)).
func SelectWaveSeedMenus(evolveDir string, count, perLane int, isProtected func(string) bool) [][]FleetCandidate {
	backlog := ReadInboxBacklog(evolveDir, isProtected)
	return ExpandWithClusterMates(SelectFleetWidthTopN(backlog, count), backlog, perLane)
}
