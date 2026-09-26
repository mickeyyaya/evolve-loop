package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func rawEntry(t *testing.T, seq int, prev, msg string) []byte {
	t.Helper()
	b, err := json.Marshal(core.LedgerEntry{
		Role: "builder", Kind: "k", Message: msg, EntrySeq: seq, PrevHash: prev,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeChain(t *testing.T, lines [][]byte, lastSeq int) *FileLedger {
	t.Helper()
	dir := t.TempDir()
	var buf []byte
	for _, ln := range lines {
		buf = append(buf, ln...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, sha256Hex(lines[len(lines)-1]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(dir)
}

func TestWalkChain_ReGenesisSeamAccepted(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	e1 := rawEntry(t, 1, sha256Hex(e0), "b")
	seam := rawEntry(t, 0, ZeroSeed, "restart") // lost-tip re-genesis
	e2 := rawEntry(t, 1, sha256Hex(seam), "c")
	l := writeChain(t, [][]byte{e0, e1, seam, e2}, 1)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("re-genesis seam must verify: %v", err)
	}
}

func TestWalkChain_ZeroPrevWithNonzeroSeqIsBroken(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	forged := rawEntry(t, 7, ZeroSeed, "forged-restart")
	l := writeChain(t, [][]byte{e0, forged}, 7)
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("zero prev with nonzero seq must stay a chain break, got: %v", err)
	}
}

func TestWalkChain_ForkSiblingAccepted(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	e1 := rawEntry(t, 1, sha256Hex(e0), "child-A")
	sib := rawEntry(t, 1, sha256Hex(e0), "child-B") // same parent as e1
	e2 := rawEntry(t, 2, sha256Hex(sib), "after-fork")
	l := writeChain(t, [][]byte{e0, e1, sib, e2}, 2)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("fork sibling must verify: %v", err)
	}
}

func TestWalkChain_ThreeForkSiblingsAccepted(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	parent := sha256Hex(e0)
	s1 := rawEntry(t, 1, parent, "racer-1")
	s2 := rawEntry(t, 1, parent, "racer-2")
	s3 := rawEntry(t, 1, parent, "racer-3")
	e2 := rawEntry(t, 2, sha256Hex(s3), "after")
	l := writeChain(t, [][]byte{e0, s1, s2, s3, e2}, 2)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("a 3-wide sibling run must verify: %v", err)
	}
}

func TestWalkChain_PrevMatchingNothingIsBroken(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	e1 := rawEntry(t, 1, sha256Hex(e0), "b")
	orphan := rawEntry(t, 2, sha256Hex([]byte("rewritten-predecessor")), "orphan")
	l := writeChain(t, [][]byte{e0, e1, orphan}, 2)
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("orphaned prev must stay a chain break, got: %v", err)
	}
}

func TestWalkChain_DuplicatePrevWithoutSiblingSignatureIsBroken(t *testing.T) {
	e0 := rawEntry(t, 0, ZeroSeed, "a")
	e1 := rawEntry(t, 1, sha256Hex(e0), "b")
	e2 := rawEntry(t, 2, sha256Hex(e1), "c")
	// Re-uses e1's parent hash but is not adjacent to e1, so it is not a sibling.
	dup := rawEntry(t, 3, sha256Hex(e0), "late-dup")
	l := writeChain(t, [][]byte{e0, e1, e2, dup}, 3)
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("non-adjacent duplicate prev must stay a chain break, got: %v", err)
	}
}
