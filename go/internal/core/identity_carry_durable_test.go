package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeCarryThroughTheRealLedger(ledgerPath string, in core.CompositionVerdictInput) error {
	return ledger.WriteCompositionVerdict(ledgerPath, ledger.CompositionVerdictInput(in))
}

func TestCarryRecord_VerifiesAfterItsCycleWorktreeIsRemoved(t *testing.T) {
	root, worktree, next := core.CarryAPendedIdenticalLaneForTesting(t, writeCarryThroughTheRealLedger)
	if next != core.PhaseShip {
		t.Fatalf("route = %s, want Ship: the byte-identical rebase must record its carry", next)
	}

	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}

	if err := ledger.New(filepath.Join(root, ".evolve")).Verify(context.Background()); err != nil {
		t.Fatalf("the project ledger stops verifying once the carry's cycle worktree is cleaned up, so ship refuses every later carry: %v", err)
	}
}
