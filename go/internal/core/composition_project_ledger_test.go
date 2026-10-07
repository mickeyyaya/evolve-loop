package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestCompositionCarry_ChainsFromTheProjectLedgersTipThroughALinkedWorktree(t *testing.T) {
	for method, rung := range map[string]func(core.CompositionRungsForTesting) func() bool{
		ledger.TrivialRebaseMethod: func(r core.CompositionRungsForTesting) func() bool { return r.TrivialRebase },
		ledger.ScopedReviewMethod:  func(r core.CompositionRungsForTesting) func() bool { return r.ScopedReview },
	} {
		t.Run(method, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			project := ledger.New(filepath.Join(root, ".evolve"))
			if err := project.Append(ctx, core.LedgerEntry{Role: "auditor", Kind: "agent_subprocess", Cycle: 42}); err != nil {
				t.Fatal(err)
			}

			lane := core.RungsOnALinkedLedgerForTesting(t, root, writeCarryThroughTheRealLedger)
			if !rung(lane)() {
				t.Fatalf("the %s rung must carry the clean rebase", method)
			}
			if err := project.Append(ctx, core.LedgerEntry{Role: "orchestrator", Kind: "ship", Cycle: 42}); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(lane.Worktree); err != nil {
				t.Fatal(err)
			}

			if err := project.Verify(ctx); err != nil {
				t.Fatalf("the carry wrote through the worktree's linked ledger, whose tip, lock and evidence store are the worktree's own, so the project chain broke: %v", err)
			}
		})
	}
}
