package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// chainLines is a valid 4-line chained ledger (genesis + 3) with each line's SHA.
func chainLines() (lines []string, sha []string) {
	g := `{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + ZeroSeed + `"}`
	a := fmt.Sprintf(`{"ts":"2026-05-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, sha256Of(g))
	b := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha256Of(a))
	c := fmt.Sprintf(`{"ts":"2026-05-01T00:03:00Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":3,"prev_hash":"%s"}`, sha256Of(b))
	lines = []string{g, a, b, c}
	for _, ln := range lines {
		sha = append(sha, sha256Of(ln))
	}
	return lines, sha
}

func bytesLines(strs []string) [][]byte {
	out := make([][]byte, len(strs))
	for i, s := range strs {
		out[i] = []byte(s)
	}
	return out
}

func TestWalkChain_NoAnchor_ByteIdenticalBehavior(t *testing.T) {
	lines, _ := chainLines()
	if _, _, _, err := walkChain(bytesLines(lines), ""); err != nil {
		t.Fatalf("valid chain must pass with no anchor: %v", err)
	}
	damaged := append([]string(nil), lines...)
	damaged[1] = `{"ts":"2026-05-01T00:01:00Z","cycle":999,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"` + ZeroSeed + `"}`
	if _, _, _, err := walkChain(bytesLines(damaged), ""); err == nil {
		t.Fatal("mid-chain damage must break with no anchor (regression guard)")
	}
}

func TestWalkChain_Anchor_SkipsPreEpochDamage(t *testing.T) {
	lines, sha := chainLines()
	damaged := append([]string(nil), lines...)
	damaged[1] = `{"ts":"2026-05-01T00:01:00Z","cycle":999,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"` + ZeroSeed + `"}`
	// Anchor at line 2 (b): intact, the first line after the damage.
	if _, lastSha, _, err := walkChain(bytesLines(damaged), sha[2]); err != nil {
		t.Fatalf("anchored walk past pre-epoch damage must pass: %v", err)
	} else if lastSha != sha[3] {
		t.Errorf("lastSha=%s, want last line sha %s", lastSha, sha[3])
	}
}

func TestWalkChain_Anchor_PostAnchorBreakStillCaught(t *testing.T) {
	lines, sha := chainLines()
	// Anchor at line 1 (a); damage line 2 (b) → line 3 (c)'s prev mismatches.
	damaged := append([]string(nil), lines...)
	damaged[2] = `{"ts":"2026-05-01T00:02:00Z","cycle":999,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"` + sha256Of(lines[1]) + `"}`
	if _, _, _, err := walkChain(bytesLines(damaged), sha[1]); err == nil {
		t.Fatal("post-anchor damage must still break the chain")
	}
}

func TestWalkChain_AnchorNotFound_Errors(t *testing.T) {
	lines, _ := chainLines()
	if _, _, _, err := walkChain(bytesLines(lines), "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatal("an anchor SHA matching no line must error")
	}
}

func TestWalkChain_AnchorLineMutated_NotFound(t *testing.T) {
	lines, sha := chainLines()
	mutated := append([]string(nil), lines...)
	mutated[2] = `{"ts":"2026-05-01T00:02:00Z","cycle":777,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"` + sha256Of(lines[1]) + `"}`
	if _, _, _, err := walkChain(bytesLines(mutated), sha[2]); err == nil {
		t.Fatal("mutating the anchor line itself must fail 'anchor not found' (SHA binding self-invalidates)")
	}
}

func TestAnchor_FindsSealedSegmentLine(t *testing.T) {
	l, dir := seedLedger(t, 10)
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// seq 2 is now in a sealed segment, absent from the live tail.
	if err := l.Anchor(context.Background(), 2, "sealed-line epoch"); err != nil {
		t.Fatalf("Anchor must find a sealed-segment line, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ledger-anchor.json")); err != nil {
		t.Fatalf("anchor file not written for a sealed seq: %v", err)
	}
}

func TestAnchor_RecordsLineSHA(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines, sha := chainLines()
	body := ""
	for _, ln := range lines {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(evolveDir)
	if err := l.Anchor(context.Background(), 2, "ledger-1740 epoch"); err != nil {
		t.Fatalf("Anchor(2): %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "ledger-anchor.json"))
	if err != nil {
		t.Fatalf("anchor file not written: %v", err)
	}
	var rec struct {
		AnchorSeq     int    `json:"anchor_seq"`
		AnchorLineSHA string `json:"anchor_line_sha256"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("anchor file invalid JSON: %v", err)
	}
	if rec.AnchorSeq != 2 || rec.AnchorLineSHA != sha[2] {
		t.Errorf("anchor rec=%+v, want seq=2 sha=%s", rec, sha[2])
	}
}

func TestAnchor_SkipsMalformedLedgerLines(t *testing.T) {
	evolveDir := t.TempDir()
	lines, _ := chainLines()
	body := "{not-json}\n" + lines[2] + "\n"
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(evolveDir)
	if err := l.Anchor(context.Background(), 2, "skip malformed line"); err != nil {
		t.Fatalf("Anchor must skip malformed lines and find seq 2: %v", err)
	}
}

func TestAnchor_SeqNotFound_Errors(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines, _ := chainLines()
	body := ""
	for _, ln := range lines {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(evolveDir)
	if err := l.Anchor(context.Background(), 99, "nope"); err == nil {
		t.Fatal("anchoring a nonexistent seq must error")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "ledger-anchor.json")); !os.IsNotExist(err) {
		t.Error("no anchor file should be written on a failed anchor")
	}
}

func TestVerify_HonorsAnchorFile(t *testing.T) {
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines, sha := chainLines()
	damaged := append([]string(nil), lines...)
	damaged[1] = `{"ts":"2026-05-01T00:01:00Z","cycle":999,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"` + ZeroSeed + `"}`
	body := ""
	for _, ln := range damaged {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.tip"), []byte(fmt.Sprintf("3:%s", sha[3])), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(evolveDir)
	if err := l.Verify(context.Background()); err == nil {
		t.Fatal("damaged chain must fail plain Verify before any anchor")
	}
	rec := fmt.Sprintf(`{"anchor_seq":2,"anchor_line_sha256":"%s","recorded_at":"2026-06-13T00:00:00Z","note":"test"}`, sha[2])
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger-anchor.json"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("Verify must honor the epoch anchor and pass: %v", err)
	}
}
