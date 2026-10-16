package triagecap

import (
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func ExpandWithClusterMates(selection, backlog []FleetCandidate, perLane int) [][]FleetCandidate {
	menus := make([][]FleetCandidate, 0, len(selection))
	owner := map[string]int{}
	inMenu := make(map[string]bool, len(selection))
	for lane, rep := range selection {
		menus = append(menus, []FleetCandidate{rep})
		inMenu[rep.ID] = true
		for _, f := range rep.Files {
			cf := filepath.Clean(f)
			if _, taken := owner[cf]; !taken {
				owner[cf] = lane
			}
		}
	}
	if perLane < 2 {
		return menus
	}
	for _, c := range backlog {
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

func SelectWaveSeedMenus(evolveDir string, committed []FleetCandidate, count, perLane int, isProtected func(string) bool) [][]FleetCandidate {
	backlog := ReadInboxBacklog(evolveDir, isProtected)
	committed = PruneUndispatchable(evolveDir, committed)
	seed := SelectFleetWidthTopN(backlog, count)
	if len(committed) > 0 {
		seed = WidenTopNToFleetWidth(committed, backlog, count)
	}
	return ExpandWithClusterMates(seed, backlog, perLane)
}

func PruneUndispatchable(evolveDir string, committed []FleetCandidate) []FleetCandidate {
	if len(committed) == 0 {
		return committed
	}
	lifecycle := readOnlyLifecycle(evolveDir)
	kept := make([]FleetCandidate, 0, len(committed))
	for _, c := range committed {
		if inboxmover.ResolveDispatchability(lifecycle, c.ID).Dispatchable {
			kept = append(kept, c)
		}
	}
	return kept
}

func readOnlyLifecycle(evolveDir string) inboxmover.Options {
	return inboxmover.Options{InboxDir: filepath.Join(evolveDir, "inbox"), Stderr: io.Discard}
}
