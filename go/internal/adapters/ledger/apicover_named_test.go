package ledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
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
