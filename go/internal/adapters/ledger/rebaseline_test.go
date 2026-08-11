package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rebaselineNote = "operator sign-off: pre-CA.1 fleet-concurrency damage accepted"

// damagedLedger writes a chain with THREE distinct prev_hash breaks in the
// prefix — the console-plane shape in miniature, where per-line `anchor` calls
// do not scale — and returns the evolve dir.
func damagedLedger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	g := `{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + ZeroSeed + `"}`
	b1 := fmt.Sprintf(`{"ts":"2026-05-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, strings.Repeat("a1", 32))
	b2 := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, strings.Repeat("b2", 32))
	b3 := fmt.Sprintf(`{"ts":"2026-05-01T00:03:00Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":3,"prev_hash":"%s"}`, strings.Repeat("c3", 32))
	lines := []string{g, b1, b2, b3}
	body := ""
	for _, ln := range lines {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := fmt.Sprintf("%d:%s", 3, sha256Of(b3))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRebaseline_SealsDamagedPrefix: ONE call turns a three-break chain from RED
// to GREEN under VerifyDeep, and the appended record is auditable — it carries
// the operator's note and identifies itself as a rebaseline rather than passing
// for an ordinary anchor.
func TestRebaseline_SealsDamagedPrefix(t *testing.T) {
	dir := damagedLedger(t)
	l := New(dir)

	// Pre-state: without this the test could pass against an already-green chain.
	if err := l.VerifyDeep(context.Background()); err == nil {
		t.Fatal("fixture precondition failed: a chain with three planted breaks verified GREEN")
	}

	if err := l.Rebaseline(context.Background(), rebaselineNote); err != nil {
		t.Fatalf("Rebaseline: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Errorf("chain still broken after ONE rebaseline: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(raw)
	var seal struct {
		Role    string `json:"role"`
		Kind    string `json:"kind"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &seal); err != nil {
		t.Fatalf("unmarshal seal line: %v", err)
	}
	if seal.Kind != RebaselineKind {
		t.Errorf("seal Kind = %q, want %q — an unrecognised kind does not move the epoch anchor", seal.Kind, RebaselineKind)
	}
	if !strings.HasPrefix(seal.Kind, resetSealKindPrefix) {
		t.Errorf("seal Kind %q lacks the in-band seal prefix effectiveAnchorSHA keys on", seal.Kind)
	}
	if seal.Role != operatorRole {
		t.Errorf("seal Role = %q, want %q — a non-operator role must not be able to silence Verify", seal.Role, operatorRole)
	}
	if seal.Message != rebaselineNote {
		t.Errorf("seal Message = %q, want the operator note %q — an unattributable trust decision", seal.Message, rebaselineNote)
	}
}

// TestRebaseline_PreservesDamagedPrefixBytes: the repair is append-only. A
// rebaseline that greened the chain by rewriting or truncating history would
// destroy the auditable record — the outcome ADR-0048 rejects in favour of the
// epoch anchor.
func TestRebaseline_PreservesDamagedPrefixBytes(t *testing.T) {
	dir := damagedLedger(t)
	path := filepath.Join(dir, "ledger.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := New(dir).Rebaseline(context.Background(), rebaselineNote); err != nil {
		t.Fatalf("Rebaseline: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) <= len(before) {
		t.Fatalf("ledger did not grow (%d -> %d bytes): the seal must be APPENDED", len(before), len(after))
	}
	if string(after[:len(before)]) != string(before) {
		t.Error("rebaseline mutated pre-existing history — the damaged prefix must be preserved byte-for-byte, only un-chain-validated")
	}
}

// TestRebaseline_RefusesUngatedAndEmptyChain: a command that seals whatever it is
// pointed at, whenever it is invoked, is a chain-integrity bypass rather than a
// repair tool. Both refusals must write nothing at all.
func TestRebaseline_RefusesUngatedAndEmptyChain(t *testing.T) {
	t.Run("missing_operator_note", func(t *testing.T) {
		for _, note := range []string{"", "   "} {
			dir := damagedLedger(t)
			path := filepath.Join(dir, "ledger.jsonl")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := New(dir).Rebaseline(context.Background(), note); err == nil {
				t.Fatalf("Rebaseline(%q) must be refused: a bulk trust decision needs its justification", note)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Error("a refused rebaseline still mutated ledger.jsonl")
			}
		}
	})

	t.Run("empty_chain_is_not_fabricated", func(t *testing.T) {
		dir := t.TempDir() // no ledger.jsonl at all
		if err := New(dir).Rebaseline(context.Background(), rebaselineNote); err == nil {
			t.Fatal("Rebaseline against a nonexistent chain must be refused, not minted")
		}
		if _, err := os.Stat(filepath.Join(dir, "ledger.jsonl")); err == nil {
			t.Error("rebaseline fabricated a ledger.jsonl where none existed")
		}
		if _, err := os.Stat(filepath.Join(dir, "ledger-anchor.json")); err == nil {
			t.Error("rebaseline wrote an anchor for a ledger that does not exist")
		}
	})
}
