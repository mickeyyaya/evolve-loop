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

// siblingLedger writes a chain in which entry_seq=2 is carried by TWO distinct
// lines — the fork-sibling shape a pre-CA.1 concurrent Append produced — and
// returns the evolve dir plus every line's SHA (index-aligned).
func siblingLedger(t *testing.T) (dir string, sha []string) {
	t.Helper()
	dir = t.TempDir()
	g := `{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + ZeroSeed + `"}`
	a := fmt.Sprintf(`{"ts":"2026-05-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, sha256Of(g))
	// Two racers off the SAME parent, both stamped entry_seq=2.
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

// TestAnchor_RejectsAmbiguousSeq: a seq carried by two distinct lines is refused
// with ErrAmbiguousAnchorSeq, naming both candidates, and writes NO anchor file.
// Before cycle-1433 this bound the FIRST sibling and exited 0, silently moving
// the epoch anchor backward past a line the operator believed was sealed.
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

// TestAnchor_RejectsAmbiguousSeq_ByteIdenticalLinesAreNotAmbiguous: distinctness
// is by SHA, not by line count. Two byte-identical lines are one set of bytes to
// bind, so the anchor still resolves — otherwise a duplicated line would make an
// anchor permanently unreachable.
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

// TestAnchor_LineSHABindsNamedSibling: --line-sha resolves the ambiguity by
// binding the exact line named — here the SECOND sibling, the one a first-match
// anchor would never have chosen — and the anchored chain verifies forward.
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

// TestAnchor_LineSHANegatives: the flag must not become a way to bind an
// arbitrary line. A SHA carrying a different seq, and a SHA present in no line,
// are both refused with no residue.
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
