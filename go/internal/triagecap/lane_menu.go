package triagecap

import (
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// ExpandWithClusterMates deepens each selected lane with same-file backlog mates, up to perLane members in all.
// A mate joins only a lane it alone overlaps, then claims its files for that lane; a candidate touching no lane
// stays width material, and one touching two lanes would bridge two worktrees, so it joins neither.
func ExpandWithClusterMates(selection, backlog []FleetCandidate, perLane int) [][]FleetCandidate {
	menus := make([][]FleetCandidate, 0, len(selection))
	owner := map[string]int{} // normalized file → owning lane index
	inMenu := make(map[string]bool, len(selection))
	for lane, rep := range selection {
		menus = append(menus, []FleetCandidate{rep})
		inMenu[rep.ID] = true
		for _, f := range rep.Files {
			cf := filepath.Clean(f)
			if _, taken := owner[cf]; !taken { // a committed prefix may overlap itself; the first lane keeps the file
				owner[cf] = lane
			}
		}
	}
	if perLane < 2 {
		return menus
	}
	sorted := rankForDispatch(backlog)
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

// soleOwningLane is ok=false for zero owners (independent work) and for two or more (a bridge).
func soleOwningLane(owner map[string]int, files []string) (lane int, ok bool) {
	lane = -1
	for _, f := range files {
		l, claimed := owner[filepath.Clean(f)]
		if !claimed {
			continue
		}
		if lane != -1 && lane != l {
			return 0, false
		}
		lane = l
	}
	return lane, lane != -1
}

// SelectWaveSeedMenus keeps the live committed prefix, widens it to count disjoint lanes, and deepens each lane.
// An empty prefix uses SelectFleetWidthTopN, which still yields one lane at count<2 where widening yields none.
func SelectWaveSeedMenus(evolveDir string, committed []FleetCandidate, count, perLane int, isProtected func(string) bool) [][]FleetCandidate {
	backlog := ReadInboxBacklog(evolveDir, isProtected)
	committed = PruneConsumed(evolveDir, committed)
	seed := SelectFleetWidthTopN(backlog, count)
	if len(committed) > 0 {
		seed = WidenTopNToFleetWidth(committed, backlog, count)
	}
	return ExpandWithClusterMates(seed, backlog, perLane)
}

// PruneConsumed drops committed ids in a terminal inbox state, since widening copies the prefix verbatim.
// An id with no lifecycle evidence stays: dropping what cannot be resolved would starve waves of non-inbox cards.
func PruneConsumed(evolveDir string, committed []FleetCandidate) []FleetCandidate {
	if len(committed) == 0 {
		return committed
	}
	opts := inboxmover.Options{InboxDir: filepath.Join(evolveDir, "inbox"), Stderr: io.Discard}
	kept := make([]FleetCandidate, 0, len(committed))
	for _, c := range committed {
		switch inboxmover.ResolveDispatchState(opts, c.ID).State {
		case inboxmover.StateProcessed, inboxmover.StateConsumed, inboxmover.StateRejected, inboxmover.StateQuarantine:
			continue
		}
		kept = append(kept, c)
	}
	return kept
}
