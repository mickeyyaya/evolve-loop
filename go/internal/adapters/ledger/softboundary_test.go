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

func TestVerify_SoftBoundary_Mixed(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pre1 := `{"ts":"2026-04-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0}`
	pre2 := `{"ts":"2026-04-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1}`

	pre2Sha := sha256Of(pre2)
	post1 := fmt.Sprintf(`{"ts":"2026-04-02T00:00:00Z","cycle":2,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, pre2Sha)

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

func TestVerify_SoftBoundary_FirstV837_WrongPrev(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pre1 := `{"ts":"x","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0}`
	// A zero seed with a nonzero seq: neither a genesis nor chained from pre1.
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
	// No tip file: an unchained ledger needs none.

	l := New(evolveDir)
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("all-pre-v8.37 ledger should verify (soft boundary), got: %v", err)
	}
}

func sha256Of(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
