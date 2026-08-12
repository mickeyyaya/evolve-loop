package deliverable

// salvage_report_saved_test.go — unit coverage for the `saved` counter's fold
// (CountSalvageApplied) and the exported sidecar name the CLI opens
// (SalvageAppliedFile). The end-to-end contract through the real binary is
// go/acs/cycle1441/predicates_test.go TestC1441_007/008; these are the
// per-branch assertions that suite cannot make cheaply.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestCountSalvageApplied_CountsOnlyOwnEventType — the sidecar is a repo-level
// file any emitter may append to, so a foreign event_type, a blank line and a
// run-tag from another process must all leave the count alone: it is "how many
// coercions did the gate perform", all runs.
func TestCountSalvageApplied_CountsOnlyOwnEventType(t *testing.T) {
	t.Parallel()
	const jsonl = `{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json","run":"111"}
{"event_type":"some_other_emitter","phase":"build","pattern":"fenced-json","run":"111"}

{"event_type":"salvage_applied","phase":"build","pattern":"trailing-comma","run":"222"}
`
	got, _, err := CountSalvageApplied(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("well-formed sidecar must not error: %v", err)
	}
	if got != 2 {
		t.Errorf("CountSalvageApplied = %d, want 2 — the foreign emitter and the blank line must not be counted", got)
	}
}

// TestCountSalvageApplied_EmptyAndTorn — an empty sidecar is 0 (the normal
// never-salvaged state); a torn append is REPORTED, not fatal.
//
// INVERTED, declared loudly (cycle-1442 audit M2). This test previously
// asserted a torn line must be "a loud error", and that literal reading is
// what the auditor tabled as the defect: `evolve salvage report` exited 1 with
// NO output over one crash-torn byte, discarding the entire already-computed
// baseline section — while the in-process summary tolerated the very same
// shape by design. Two consumers of one unauthenticated file disagreeing on
// robustness was the finding. Loudness is preserved where it belongs: the
// skipped count is RETURNED and the CLI prints a WARN naming it, so tolerance
// can never quietly hide records.
func TestCountSalvageApplied_EmptyAndTorn(t *testing.T) {
	t.Parallel()
	if got, malformed, err := CountSalvageApplied(strings.NewReader("\n\n")); err != nil || got != 0 || malformed != 0 {
		t.Errorf("empty sidecar: got (%d, %d, %v), want (0, 0, nil)", got, malformed, err)
	}
	got, malformed, err := CountSalvageApplied(strings.NewReader(`{"event_type":"salvage_app`))
	if err != nil {
		t.Errorf("a torn line must not brick the count: %v", err)
	}
	if got != 0 || malformed != 1 {
		t.Errorf("torn line: got (saved=%d, malformed=%d), want (0, 1) — the skip must be counted, not silent", got, malformed)
	}
}

// TestSalvageAppliedFile_IsTheFileTheGateWrites — the exported name exists so
// the CLI opens the very file recordSalvageApplied appends to. Asserted by
// writing through the production recorder and reading back at the exported
// name, so the two can never drift into separate string literals.
func TestSalvageAppliedFile_IsTheFileTheGateWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordSalvageApplied(phasecontract.Roots{EvolveDir: dir}, "audit", SalvagePatternFencedJSON, nil)

	f, err := os.Open(filepath.Join(dir, SalvageAppliedFile))
	if err != nil {
		t.Fatalf("recordSalvageApplied did not write %s: %v", SalvageAppliedFile, err)
	}
	defer f.Close()
	got, _, err := CountSalvageApplied(f)
	if err != nil {
		t.Fatalf("fold the recorder's own output: %v", err)
	}
	if got != 1 {
		t.Errorf("saved = %d after one recorded salvage, want 1", got)
	}
}
