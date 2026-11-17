package loopwave

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreempt_AnUnwritableLedgerWarnsAndStillPlansTheSwap(t *testing.T) {
	h := newHarness(t)
	for _, it := range []preemptItem{{id: "slot-low", weight: 0.3}, {id: "keep-high", weight: 0.8}, {id: "fresh-top", weight: 0.95}} {
		writePreemptItem(t, filepath.Join(h.evolveDir, "inbox"), it)
	}
	withPriorDecision(t, h, "slot-low", "keep-high")
	if err := os.MkdirAll(filepath.Join(h.evolveDir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	data, _, err := selectorEngine(h, Selector{}).PlanFn(2)(context.Background(), 1)

	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if ids := topNIDs(t, data); !ids["fresh-top"] || ids["slot-low"] {
		t.Errorf("planned ids = %v, want the swap planned though its ledger line failed", ids)
	}
	if !strings.Contains(h.stderr.String(), "the preempt line stays unwritten") {
		t.Errorf("stderr = %q, want the ledger failure named", h.stderr.String())
	}
}
