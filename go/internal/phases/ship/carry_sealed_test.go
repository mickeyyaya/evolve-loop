package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func laneLedger(l carriedLane) *ledger.FileLedger {
	return ledger.New(filepath.Join(l.repo, ".evolve"))
}

func appendHistory(t *testing.T, l carriedLane, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := laneLedger(l).Append(context.Background(), core.LedgerEntry{
			Cycle: 1700 + i, Role: "builder", Kind: "phase_complete", Message: fmt.Sprintf("history-%d", i),
		}); err != nil {
			t.Fatalf("Append history %d: %v", i, err)
		}
	}
}

func sealLaneLedger(t *testing.T, l carriedLane, keepTail int) {
	t.Helper()
	if err := laneLedger(l).Seal(context.Background(), keepTail); err != nil {
		t.Fatalf("Seal(%d): %v", keepTail, err)
	}
}

func dropFirstLiveLine(t *testing.T, l carriedLane) {
	t.Helper()
	ledgerPath := filepath.Join(l.repo, ".evolve", "ledger.jsonl")
	body, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(body), "\n")
	if !found || rest == "" {
		t.Fatal("fixture: the live tail has no second line to keep")
	}
	if err := os.WriteFile(ledgerPath, []byte(rest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCarrySatisfied_ReProvesOnASealedLedger(t *testing.T) {
	ctx := context.Background()
	t.Run("the carry is the line the seal keeps live", func(t *testing.T) {
		l := rebasedLane(t)
		appendHistory(t, l, 8)
		writeCarry(t, l, "audit-ref", l.tree0)
		sealLaneLedger(t, l, 1)

		ok, detail := carrySatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1)

		if !ok || !strings.Contains(detail, "carry of cycle 1715, re-proven") {
			t.Fatalf("carrySatisfied = (%v, %q); sealing the history under a byte-identical carry must not stop it re-proving", ok, detail)
		}
	})
	t.Run("the carry is written after a seal", func(t *testing.T) {
		l := rebasedLane(t)
		appendHistory(t, l, 8)
		sealLaneLedger(t, l, 2)
		writeCarry(t, l, "audit-ref", l.tree0)

		ok, detail := carrySatisfied(ctx, boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1)

		if !ok || !strings.Contains(detail, "carry of cycle 1715, re-proven") {
			t.Fatalf("carrySatisfied = (%v, %q); a carry recorded on an already sealed ledger must re-prove", ok, detail)
		}
	})
}

func TestCarrySatisfied_DeclinesWhenASealedLedgersLiveTailLostALine(t *testing.T) {
	l := rebasedLane(t)
	appendHistory(t, l, 8)
	sealLaneLedger(t, l, 3)
	writeCarry(t, l, "audit-ref", l.tree0)
	dropFirstLiveLine(t, l)

	ok, reason := carrySatisfied(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), l.repo, l.tree1)

	if ok {
		t.Fatal("carrySatisfied re-proved a carry on a sealed ledger whose first live line was removed: the seam between the newest segment and the live tail is not verified")
	}
	if !strings.Contains(reason, "ledger") {
		t.Errorf("the decline reason %q does not name the ledger, the check that must fail", reason)
	}
}
