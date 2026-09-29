package gc

// worktrees_fuzz_test.go — property-based RED test (cycle 570,
// workspace-hygiene-s4-worktree-gc-planner) mirroring
// TestPlanNeverTouchesLiveDirs_Property (discover_fuzz_test.go): random
// synthetic worktree populations -> PlanWorktrees -> the three invariants
// that must hold no matter what the random draw produces.
//
// RED now: PlanWorktrees/WorktreeOptions do not exist yet (compile failure).
// Do NOT modify this file.

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"pgregory.net/rapid"
)

// TestPlanWorktreesNeverTouchesLiveDirtyUnmerged_Property draws a random
// population of cycle-prefixed worktrees (random merged/dirty/live/age
// combinations) and asserts PlanWorktrees NEVER emits a removal or
// branch-delete action for a live, dirty, or unmerged candidate — the same
// three invariants the named unit tests pin individually, checked here
// against many more combinations than a human would hand-write.
func TestPlanWorktreesNeverTouchesLiveDirtyUnmerged_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		e := newWorktreesTestEnv(t)

		n := rapid.IntRange(1, 10).Draw(rt, "n")
		type want struct {
			merged, dirty, live, closed bool
			closedAgo                   time.Duration
		}
		expect := map[string]want{}

		for i := 0; i < n; i++ {
			leaf := fmt.Sprintf("cycle-fuzz%d-%d", i, 900+i)
			merged := rapid.Bool().Draw(rt, "merged-"+leaf)
			dirty := rapid.Bool().Draw(rt, "dirty-"+leaf)
			live := rapid.Bool().Draw(rt, "live-"+leaf)
			age := time.Duration(rapid.IntRange(1, 60*24).Draw(rt, "age-"+leaf)) * time.Minute

			closed := rapid.Bool().Draw(rt, "closed-"+leaf)
			closedAgo := time.Duration(rapid.IntRange(0, 72).Draw(rt, "closed-ago-"+leaf)) * time.Hour

			path := e.addWorktree(leaf, leaf, age, dirty, merged)
			if live {
				e.writeLease(900+i, runlease.Lease{RunID: leaf}, 1*time.Minute)
			}
			if closed {
				closeOutCycle(t, e.projectRoot, 900+i, e.now.Add(-closedAgo))
			}
			expect[path] = want{merged: merged, dirty: dirty, live: live, closed: closed, closedAgo: closedAgo}
		}
		o := e.opts()
		o.Policy.SalvageAfterHours = rapid.IntRange(0, 48).Draw(rt, "salvage-after-hours")

		m, err := PlanWorktrees(o)
		if err != nil {
			rt.Fatalf("PlanWorktrees: %v", err)
		}

		removedOrDeleted := map[string]WorktreeAction{}
		for _, it := range m.Items {
			if it.Action == WorktreeActionSalvageRemove {
				w := expect[it.Path]
				if w.live || !w.closed || o.Policy.SalvageAfterHours == 0 || w.closedAgo < time.Duration(o.Policy.SalvageAfterHours)*time.Hour {
					rt.Fatalf("salvage-remove of %s outside a finished, policy-aged, dead tree: %+v policy=%dh", it.Path, w, o.Policy.SalvageAfterHours)
				}
			}
			if it.Action == WorktreeActionRemove || it.Action == WorktreeActionDeleteBranch {
				key := it.Path
				if key == "" {
					key = filepath.Join(e.worktreeBase, it.Branch)
				}
				removedOrDeleted[key] = it.Action
			}
		}

		for path, w := range expect {
			if action, touched := removedOrDeleted[path]; touched {
				if w.live {
					rt.Fatalf("live path %s must never be acted on (got %s)", path, action)
				}
				if w.dirty {
					rt.Fatalf("dirty path %s must never be acted on (got %s)", path, action)
				}
				if !w.merged {
					rt.Fatalf("unmerged path %s must never be acted on (got %s)", path, action)
				}
			}
		}
	})
}
