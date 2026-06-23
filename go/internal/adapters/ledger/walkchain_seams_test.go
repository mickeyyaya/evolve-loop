package ledger

// Seam semantics for walkChain (L3.3): the production ledger's history
// contains benign artifacts the strict walk wrongly rejected — making
// plain `evolve ledger verify` red on the real file. Pin exactly which
// classes are accepted and which remain chain breaks:
//
//	ACCEPTED: re-genesis seam (zero prev + seq 0, the lost-tip restart)
//	ACCEPTED: fork sibling (prev equals the PREVIOUS line's prev — the
//	          pre-CA.1 concurrent-Append race)
//	BROKEN:   zero prev with a nonzero seq (a forged restart)
//	BROKEN:   prev matching nothing (the line-1740 class: predecessor
//	          bytes rewritten post-hoc)
//	BROKEN:   duplicate prev without the sibling signature

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// rawEntry marshals a LedgerEntry with explicit seq/prev into one line.
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

// writeChain writes lines + a consistent tip and returns the ledger.
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
	sib := rawEntry(t, 1, sha256Hex(e0), "child-B") // same parent: pre-CA.1 race
	e2 := rawEntry(t, 2, sha256Hex(sib), "after-fork")
	l := writeChain(t, [][]byte{e0, e1, sib, e2}, 2)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("fork sibling must verify: %v", err)
	}
}

// Sibling runs are unbounded by design: a wider pre-CA.1 race wrote 3+
// entries off one parent; each is adjacent to the previous and shares its
// parent, so the same signature accepts the whole run.
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
	// The line-1740 class: chains from a hash that matches neither the
	// previous line nor its parent.
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
	// Re-uses e1's parent hash but is NOT adjacent to e1 — not a sibling.
	dup := rawEntry(t, 3, sha256Hex(e0), "late-dup")
	l := writeChain(t, [][]byte{e0, e1, e2, dup}, 3)
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("non-adjacent duplicate prev must stay a chain break, got: %v", err)
	}
}
