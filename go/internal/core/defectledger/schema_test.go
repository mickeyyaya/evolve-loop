package defectledger

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test 7 — Read: absent → (zero, false, nil); a directory at the path → the
// `read defect-ledger.json:` fault; garbage → `parse defect-ledger.json:`;
// Write renders the G1 bytes; a round-trip is lossless; OpenEntries is the
// exact compare (a padded " OPEN" is excluded).
func TestRead_AbsentUnreadableMalformed_AndWrite_GoldenBytes(t *testing.T) {
	dir := t.TempDir()
	if doc, ok, err := Read(dir); ok || err != nil || doc.OriginCycle != 0 || doc.Entries != nil {
		t.Fatalf("absent: %+v %v %v", doc, ok, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, LedgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	_, ok, err := Read(dir)
	var pathErr *fs.PathError
	if ok || err == nil || !strings.HasPrefix(err.Error(), "read defect-ledger.json: ") || !errors.As(err, &pathErr) {
		t.Fatalf("a directory at the path is a read fault wrapping the os error: %v %v", ok, err)
	}
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LedgerFile), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Read(dir); ok || err == nil || !strings.HasPrefix(err.Error(), "parse defect-ledger.json: ") {
		t.Fatalf("garbage is a parse fault: %v %v", ok, err)
	}

	golden := goldenBytes(t, "emit_ledger.golden.json")
	var doc Doc
	if err := json.Unmarshal(golden, &doc); err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	if err := Write(dir, doc); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, LedgerFile)); got != string(golden) {
		t.Fatalf("Write renders the golden bytes:\n%s", got)
	}
	back, ok, err := Read(dir)
	if err != nil || !ok || back.OriginCycle != 1255 || len(back.Entries) != 4 || back.Entries[3].Text != PrescriptionPrefix+"add a regression test" {
		t.Fatalf("round-trip: %+v %v %v", back, ok, err)
	}
	if err := Write(filepath.Join(dir, "nested", "deeper"), doc); err != nil {
		t.Fatalf("Write creates the directory: %v", err)
	}

	d := Doc{Entries: []Entry{{ID: "a", Status: StatusOpen}, {ID: "b", Status: StatusFixed}, {ID: "c", Status: " OPEN"}, {ID: "d", Status: StatusOpen}}}
	open := d.OpenEntries()
	if len(open) != 2 || open[0].ID != "a" || open[1].ID != "d" {
		t.Fatalf("OpenEntries is the exact compare: %+v", open)
	}
}

// Test 8 — the id is "d" + hex(sha256[:16]): 33 runes, distinct texts distinct.
func TestID_IsPrefixedSHA256Sixteen(t *testing.T) {
	id := ID("some defect text")
	if len(id) != 33 || id[0] != 'd' || ID("a") == ID("b") {
		t.Fatalf("ID = %q", id)
	}
	if ID("alpha defect") != "d32047ebed901dcb546b8d7b43e2888d6" {
		t.Fatalf("the golden id of %q: %s", "alpha defect", ID("alpha defect"))
	}
}

// Test 9 — the FOURTH rune-cap rule: no TrimSpace, cut at max, the suffix
// `…[truncated]` with no leading space; the three carryover rules (CapRunes
// `…`, TruncateRunes ` …[truncated]` after TrimSpace, Summary ` ...[truncated]`)
// all differ from it on the same input.
func TestTruncate_FourthRuleVerbatim(t *testing.T) {
	if got := Truncate("  ab", 10); got != "  ab" {
		t.Fatalf("under the cap is returned verbatim, untrimmed: %q", got)
	}
	if got := Truncate("héllo wörld", 5); got != "héllo…[truncated]" {
		t.Fatalf("cut at max runes with the audit suffix: %q", got)
	}
	if got := Truncate("  abcdef", 4); got != "  ab…[truncated]" {
		t.Fatalf("no TrimSpace before the cut: %q", got)
	}
	for name, sibling := range map[string]string{
		"carryover.CapRunes":      "  ab…",
		"carryover.TruncateRunes": "abcd …[truncated]",
		"carryover.Summary":       "  ab ...[truncated]",
	} {
		if Truncate("  abcdef", 4) == sibling {
			t.Fatalf("the audit cap must stay distinct from %s", name)
		}
	}
}

// Test 30 — the written-back origin cycle is the ancestor's, or the current
// ledger's when the ancestor carries none.
func TestOriginCycleOf_FallsBackToCurrentWhenAncestorIsZero(t *testing.T) {
	if got := originCycleOf(Doc{OriginCycle: 1250}, Doc{OriginCycle: 1270}); got != 1250 {
		t.Fatalf("ancestor wins: %d", got)
	}
	if got := originCycleOf(Doc{}, Doc{OriginCycle: 1270}); got != 1270 {
		t.Fatalf("current when the ancestor is zero: %d", got)
	}
}
