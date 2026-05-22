package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Pre-v8.37 ledger entries lack a prev_hash field entirely. The bash
// verify-ledger-chain.sh treats those lines as a soft-start boundary:
// they are not retro-validated, but their SHA is computed so the first
// v8.37+ entry can chain from the last pre-v8.37 line.
//
// This test fabricates a ledger that begins with two pre-v8.37 entries
// (no prev_hash field) followed by a v8.37+ entry whose prev_hash is
// SHA256 of the second pre-v8.37 line. Verify must accept it.
func TestVerify_SoftBoundary_Mixed(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Two pre-v8.37 lines (no prev_hash field).
	pre1 := `{"ts":"2026-04-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0}`
	pre2 := `{"ts":"2026-04-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1}`

	// First v8.37+ line: prev_hash = SHA256 of pre2's bytes, entry_seq=2.
	pre2Sha := sha256Of(pre2)
	post1 := fmt.Sprintf(`{"ts":"2026-04-02T00:00:00Z","cycle":2,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, pre2Sha)

	// Second v8.37+ line: chains from post1.
	post1Sha := sha256Of(post1)
	post2 := fmt.Sprintf(`{"ts":"2026-04-02T00:01:00Z","cycle":2,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":3,"prev_hash":"%s"}`, post1Sha)

	body := pre1 + "\n" + pre2 + "\n" + post1 + "\n" + post2 + "\n"
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	post2Sha := sha256Of(post2)
	tip := fmt.Sprintf("3:%s", post2Sha)
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New(evolveDir)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("soft-boundary ledger should verify, got: %v", err)
	}
}

// A v8.37 entry whose prev_hash does NOT equal the SHA of the last
// pre-v8.37 line must still be flagged as a chain break.
func TestVerify_SoftBoundary_FirstV837_WrongPrev(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pre1 := `{"ts":"x","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0}`
	// Bad first v8.37: prev_hash = ZeroSeed (not the SHA of pre1).
	bad := fmt.Sprintf(`{"ts":"y","cycle":2,"role":"builder","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, ZeroSeed)
	badSha := sha256Of(bad)

	body := pre1 + "\n" + bad + "\n"
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.tip"), []byte(fmt.Sprintf("1:%s", badSha)), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New(evolveDir)
	if err := l.Verify(context.Background()); err == nil {
		t.Errorf("first v8.37 entry with wrong prev_hash should break verify, got nil")
	}
}

// All-pre-v8.37 ledgers verify (the soft-start boundary effectively
// covers the whole file; nothing to chain-check, no tip needed).
func TestVerify_SoftBoundary_AllPreV837(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pre1 := `{"ts":"x","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0}`
	pre2 := `{"ts":"y","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1}`
	body := pre1 + "\n" + pre2 + "\n"
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// No tip file — bash convention for pre-v8.37-only ledgers.

	l := New(evolveDir)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("all-pre-v8.37 ledger should verify (soft boundary), got: %v", err)
	}
}

func sha256Of(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
