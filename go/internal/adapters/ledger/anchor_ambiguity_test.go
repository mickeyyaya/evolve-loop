package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// siblingLedger has two fork-sibling lines carrying entry_seq=2 and returns each line's SHA, index-aligned.
func siblingLedger(t *testing.T) (dir string, sha []string) {
	t.Helper()
	dir = t.TempDir()
	g := `{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + ZeroSeed + `"}`
	a := fmt.Sprintf(`{"ts":"2026-05-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, sha256Of(g))
	b1 := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha256Of(a))
	b2 := fmt.Sprintf(`{"ts":"2026-05-01T00:02:01Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha256Of(a))
	c := fmt.Sprintf(`{"ts":"2026-05-01T00:03:00Z","cycle":1,"role":"ship","kind":"phase","exit_code":0,"entry_seq":3,"prev_hash":"%s"}`, sha256Of(b2))
	lines := []string{g, a, b1, b2, c}
	body := ""
	for _, ln := range lines {
		body += ln + "\n"
		sha = append(sha, sha256Of(ln))
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Verify checks the tip once the chain is anchored, so the fixture needs one.
	tip := fmt.Sprintf("%d:%s", 3, sha256Of(c))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, sha
}

func anchorFileExists(t *testing.T, dir string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, "ledger-anchor.json"))
	return err == nil
}

func readAnchorRec(t *testing.T, dir string) ledgerAnchor {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ledger-anchor.json"))
	if err != nil {
		t.Fatalf("anchor file not written: %v", err)
	}
	var rec ledgerAnchor
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("anchor file invalid JSON: %v", err)
	}
	return rec
}

func TestAnchor_RejectsAmbiguousSeq(t *testing.T) {
	dir, sha := siblingLedger(t)
	err := New(dir).Anchor(context.Background(), 2, "ambiguous")
	if err == nil {
		t.Fatal("anchoring a seq carried by two distinct lines must be refused, not silently bound to the first")
	}
	if !errors.Is(err, ErrAmbiguousAnchorSeq) {
		t.Errorf("error = %v, want it to wrap ErrAmbiguousAnchorSeq (the CLI keys its --line-sha remedy off this)", err)
	}
	for _, want := range []string{"entry_seq=2", sha[2], sha[3]} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q — the operator cannot act on it: %v", want, err)
		}
	}
	if anchorFileExists(t, dir) {
		t.Error("a refused ambiguous anchor still wrote ledger-anchor.json — a failed trust decision must not half-apply")
	}
}

// This anchor must resolve: byte-identical lines share one SHA, so they are one line to bind.
func TestAnchor_RejectsAmbiguousSeq_ByteIdenticalLinesAreNotAmbiguous(t *testing.T) {
	dir := t.TempDir()
	lines, sha := chainLines()
	body := lines[0] + "\n" + lines[1] + "\n" + lines[2] + "\n" + lines[2] + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(dir).Anchor(context.Background(), 2, "duplicate bytes"); err != nil {
		t.Fatalf("two byte-identical lines are not an ambiguity (same SHA to bind): %v", err)
	}
	if got := readAnchorRec(t, dir).AnchorLineSHA; got != sha[2] {
		t.Errorf("anchor_line_sha256 = %q, want %q", got, sha[2])
	}
}

func TestAnchor_LineSHABindsNamedSibling(t *testing.T) {
	dir, sha := siblingLedger(t)
	l := New(dir)
	if err := l.AnchorLine(context.Background(), 2, sha[3], "second sibling"); err != nil {
		t.Fatalf("AnchorLine with an exact SHA: %v", err)
	}
	rec := readAnchorRec(t, dir)
	if rec.AnchorLineSHA != sha[3] || rec.AnchorSeq != 2 {
		t.Errorf("anchor rec = %+v, want seq=2 sha=%s (the SECOND seq-2 line)", rec, sha[3])
	}
	if err := l.Verify(context.Background()); err != nil {
		t.Errorf("chain does not verify forward from the disambiguated anchor: %v", err)
	}
}

func TestAnchor_LineSHANegatives(t *testing.T) {
	tests := []struct {
		name    string
		sha     func(sha []string) string
		wantMsg string
	}{
		{
			name:    "sha_carries_a_different_seq",
			sha:     func(s []string) string { return s[1] }, // entry_seq 1
			wantMsg: "entry_seq=1",
		},
		{
			name:    "sha_present_in_no_line",
			sha:     func([]string) string { return strings.Repeat("de", 32) },
			wantMsg: "no line with line SHA",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, sha := siblingLedger(t)
			err := New(dir).AnchorLine(context.Background(), 2, tc.sha(sha), "bad sha")
			if err == nil {
				t.Fatal("a --line-sha that does not name a line carrying <entry_seq> must be refused")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want it to mention %q", err, tc.wantMsg)
			}
			if anchorFileExists(t, dir) {
				t.Error("a refused --line-sha still wrote ledger-anchor.json — later verifies would fail 'anchor not found' forever")
			}
		})
	}
}
