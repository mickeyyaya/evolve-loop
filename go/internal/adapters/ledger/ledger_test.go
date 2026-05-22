package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"pgregory.net/rapid"
)

func newLedger(t *testing.T) (*FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return New(evolveDir), evolveDir
}

const zero64 = "0000000000000000000000000000000000000000000000000000000000000000"

// First Append: prev_hash zero-init, entry_seq=0, tip file shows 0:sha256.
func TestAppend_FirstEntry(t *testing.T) {
	l, dir := newLedger(t)
	e := core.LedgerEntry{Role: "scout", Cycle: 1, Kind: "phase"}
	if err := l.Append(context.Background(), e); err != nil {
		t.Fatalf("append: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if !strings.Contains(string(raw), `"prev_hash":"`+zero64+`"`) {
		t.Errorf("first entry prev_hash not zero-init: %s", raw)
	}
	if !strings.Contains(string(raw), `"entry_seq":0`) {
		t.Errorf("first entry seq != 0: %s", raw)
	}
	tipBytes, err := os.ReadFile(filepath.Join(dir, "ledger.tip"))
	if err != nil {
		t.Fatalf("tip: %v", err)
	}
	tip := strings.TrimSpace(string(tipBytes))
	parts := strings.SplitN(tip, ":", 2)
	if len(parts) != 2 || parts[0] != "0" || len(parts[1]) != 64 {
		t.Errorf("tip malformed: %q", tip)
	}
}

// Second Append: prev_hash equals SHA256 of the first line as written.
func TestAppend_ChainsToPrior(t *testing.T) {
	l, dir := newLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	first, _ := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	firstLine := strings.TrimRight(string(first), "\n")
	wantPrev := sha256Hex(firstLine)

	if err := l.Append(context.Background(), core.LedgerEntry{Role: "build", Cycle: 1}); err != nil {
		t.Fatalf("append 2nd: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[1], `"prev_hash":"`+wantPrev+`"`) {
		t.Errorf("second prev_hash wrong\nline=%s\nwant prev=%s", lines[1], wantPrev)
	}
	if !strings.Contains(lines[1], `"entry_seq":1`) {
		t.Errorf("second entry_seq not 1: %s", lines[1])
	}
}

// Verify on intact chain — no error.
func TestVerify_Intact(t *testing.T) {
	l, _ := newLedger(t)
	for i := 0; i < 5; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: i + 1}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("Verify on intact chain returned %v", err)
	}
}

// Mutate a line — verify must detect.
func TestVerify_DetectsTampering(t *testing.T) {
	l, dir := newLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "build", Cycle: 1})

	path := filepath.Join(dir, "ledger.jsonl")
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// Tamper with the first line — swap role.
	lines[0] = strings.Replace(lines[0], `"role":"scout"`, `"role":"FORGED"`, 1)
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("Verify tampered chain: err=%v, want ErrLedgerChainBroken", err)
	}
}

// Tip file missing → verify returns chain-broken (tip is required after first append).
func TestVerify_TipMissing(t *testing.T) {
	l, dir := newLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	if err := os.Remove(filepath.Join(dir, "ledger.tip")); err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("Verify missing tip: err=%v, want ErrLedgerChainBroken", err)
	}
}

// Tip mismatch → verify returns chain-broken.
func TestVerify_TipMismatch(t *testing.T) {
	l, dir := newLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "scout", Cycle: 1})
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte("99:bad"+strings.Repeat("0", 61)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("Verify mismatched tip: err=%v, want ErrLedgerChainBroken", err)
	}
}

// Empty ledger (no file) → verify returns nil (bootstrap state).
func TestVerify_EmptyLedger(t *testing.T) {
	l, _ := newLedger(t)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("Verify empty: %v", err)
	}
}

// Iter yields entries in append order with prev_hash chained.
func TestIter_Order(t *testing.T) {
	l, _ := newLedger(t)
	for i := 0; i < 3; i++ {
		_ = l.Append(context.Background(), core.LedgerEntry{Role: "x", Cycle: i + 1})
	}
	it, err := l.Iter(context.Background())
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	defer it.Close()

	var seen []int
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatalf("iter.Next: %v", err)
		}
		if !ok {
			break
		}
		seen = append(seen, e.EntrySeq)
	}
	if len(seen) != 3 {
		t.Fatalf("got %d entries, want 3", len(seen))
	}
	for i, s := range seen {
		if s != i {
			t.Errorf("seen[%d]=%d, want %d", i, s, i)
		}
	}
}

// Iter on missing ledger file → empty (no error).
func TestIter_MissingFile(t *testing.T) {
	l, _ := newLedger(t)
	it, err := l.Iter(context.Background())
	if err != nil {
		t.Fatalf("iter missing file: %v", err)
	}
	defer it.Close()
	if _, ok, err := it.Next(); err != nil {
		t.Errorf("Next on empty: %v", err)
	} else if ok {
		t.Error("Next on empty returned an entry")
	}
}

// Property test: any sequence of appends produces a verifiable chain.
func TestVerify_PropertyRapid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		evolveDir := filepath.Join(dir, ".evolve")
		_ = os.MkdirAll(evolveDir, 0o755)
		l := New(evolveDir)
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		for i := 0; i < n; i++ {
			role := rapid.SampledFrom([]string{"scout", "tdd", "build", "audit"}).Draw(rt, "role")
			cycle := rapid.IntRange(1, 1000).Draw(rt, "cycle")
			if err := l.Append(context.Background(), core.LedgerEntry{Role: role, Cycle: cycle}); err != nil {
				rt.Fatalf("append: %v", err)
			}
		}
		if err := l.Verify(context.Background()); err != nil {
			rt.Errorf("Verify on synthetic chain failed: %v", err)
		}
	})
}

// Auto-detect duplicate prev_hash anomaly.
func TestVerify_DuplicatePrevHash(t *testing.T) {
	l, dir := newLedger(t)
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "a", Cycle: 1})
	_ = l.Append(context.Background(), core.LedgerEntry{Role: "b", Cycle: 1})

	// Inject a third line that re-uses the second entry's prev_hash.
	path := filepath.Join(dir, "ledger.jsonl")
	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// Copy the prev_hash from line 1 (second entry) and produce a forged line 2.
	dupLine := strings.Replace(lines[1], `"role":"b"`, `"role":"forged"`, 1)
	combined := strings.Join(append(lines, dupLine), "\n") + "\n"
	_ = os.WriteFile(path, []byte(combined), 0o644)

	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("Verify duplicate prev_hash: err=%v, want ErrLedgerChainBroken", err)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
