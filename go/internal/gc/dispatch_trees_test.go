package gc

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func treeRecord(id string, members ...proctree.Process) proctree.TreeRecord {
	return proctree.TreeRecord{File: "/t/" + id + ".json", Dispatch: id, Members: proctree.Identities(members)}
}

func TestStaleTreeFiles_AppliesTheLivenessRule(t *testing.T) {
	t.Parallel()
	deadID := "01R/1835/build/p999n1"
	reapable := gcProc(10, 1, "node", dispatchEnv(deadID), "node")
	otherStart := reapable
	otherStart.Started = reapable.Started.Add(time.Hour)
	sharedLive := gcProc(60, 1, "claude", dispatchEnv(deadID), "claude", "daemon", "run")
	cases := []struct {
		name  string
		trees []proctree.TreeRecord
		table []proctree.Process
		plan  []proctree.Process
		stale bool
	}{
		{"owner dead and no member alive", []proctree.TreeRecord{treeRecord(deadID, reapable)}, nil, nil, true},
		{"owner alive", []proctree.TreeRecord{treeRecord("01R/1835/build/p4242n1", reapable)}, nil, nil, false},
		{"a reapable member is alive", []proctree.TreeRecord{treeRecord(deadID, reapable)}, []proctree.Process{reapable}, nil, false},
		{"the member pid was reused with another start", []proctree.TreeRecord{treeRecord(deadID, reapable)}, []proctree.Process{otherStart}, nil, true},
		{"a planned member counts as dead", []proctree.TreeRecord{treeRecord(deadID, reapable)}, []proctree.Process{reapable}, []proctree.Process{reapable}, true},
		{"a live shared member does not hold the file", []proctree.TreeRecord{treeRecord(deadID, sharedLive)}, []proctree.Process{sharedLive}, nil, true},
		{"an id that does not parse", []proctree.TreeRecord{{File: "/t/x.json", Dispatch: "x"}}, nil, nil, false},
	}
	for _, tc := range cases {
		got := StaleTreeFiles(tc.trees, tc.table, staleOptions(), tc.plan)
		if (len(got) == 1) != tc.stale {
			t.Errorf("%s: stale=%v, want %v", tc.name, got, tc.stale)
		}
	}
	two := StaleTreeFiles([]proctree.TreeRecord{treeRecord(deadID, reapable), treeRecord("01R/1836/build/p999n2")}, nil, staleOptions(), nil)
	if len(two) != 2 {
		t.Errorf("two dead files: %d stale, want 2", len(two))
	}
}
