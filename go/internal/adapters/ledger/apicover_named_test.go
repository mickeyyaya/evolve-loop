package ledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestSeal_AnchorKindIsSealKind names the ledger.SealKind const and pins the
// write-side contract (seal.go:168): Seal appends a chained anchor entry whose
// Kind is SealKind — the marker VerifyDeep and the resume paths key on to bind a
// segment to its anchor.
func TestSeal_AnchorKindIsSealKind(t *testing.T) {
	l, dir := seedLedger(t, 20)
	if err := l.Seal(context.Background(), 5); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	liveRaw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(liveRaw)
	var anchor core.LedgerEntry
	if err := json.Unmarshal(lines[len(lines)-1], &anchor); err != nil {
		t.Fatalf("unmarshal anchor: %v", err)
	}
	if anchor.Kind != SealKind {
		t.Errorf("anchor Kind = %q, want %q", anchor.Kind, SealKind)
	}
}

// TestTrivialRebaseMethod_WriterReaderContract names ledger.TrivialRebaseMethod
// and pins the exact wire value the composition-verdict writer stamps into
// compositionRecord.Method (composition.go:156) and the ship-side fast path
// filters on. Writer and reader share this one constant on purpose; a drift
// silently breaks the RUNG-0 trivial-rebase audit carry-forward.
func TestTrivialRebaseMethod_WriterReaderContract(t *testing.T) {
	if TrivialRebaseMethod != "trivial-rebase" {
		t.Errorf("TrivialRebaseMethod = %q, want %q", TrivialRebaseMethod, "trivial-rebase")
	}
}
