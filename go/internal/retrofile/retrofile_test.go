// White-box behavioral suite for the retro→inbox preventive-actions
// autofiler (task retro-preventive-actions-autofile-inbox, cycle 657).
//
// These tests encode the acceptance criteria for the NEW leaf package
// internal/retrofile, which closes the learning→action loop: a retro's
// structured "preventive_actions" become weighted .evolve/inbox todos on the
// SAME deterministic seam the FAILED_UNEXPLAINED classifier already uses
// (cmd/evolve/cmd_loop_outcome.go:fileUnexplainedOutcomeDefect). They are the
// system-under-test for the cycle-657 ACS predicates
// (go/acs/cycle657/predicates_test.go), which shell `go test -run <name>
// ./internal/retrofile/` — so a predicate greens only when the real injector
// files/deduplicates the right items, never on a source-grep.
//
// RED until the Builder writes retrofile.go implementing:
//
//	type PreventiveAction struct {
//	    ID, Title  string
//	    WeightHint float64  // >0 overrides defaultWeight (recurrence hint)
//	    Files      []string
//	    Evidence   string
//	    Recurrence int
//	}
//	func ParsePreventiveActions(report []byte) ([]PreventiveAction, error)
//	func FileActions(inboxDir string, cycle int, actions []PreventiveAction,
//	    defaultWeight float64, now time.Time) (written []string, err error)
//
// DO NOT modify this file — implement production code to make it GREEN.
package retrofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow is a deterministic timestamp so filed items are byte-stable.
var fixedNow = time.Date(2026, 7, 10, 14, 30, 0, 0, time.UTC)

// fixtureRetro is a retro-report fragment carrying a machine-readable
// preventive_actions block under the "## Recommended preventive actions"
// heading — the FORMAT change the retro agent doc gains this cycle.
const fixtureRetro = "# Cycle 640 Retrospective\n\n" +
	"## Recommended preventive actions\n\n" +
	"```json\n" +
	`[
  {"id": "builder-task-binding-topn-gate", "title": "Block out-of-lane builds at the build->audit transition", "weight_hint": 0.92, "files": ["go/internal/topngate"], "evidence": "audit-report.md#D1", "recurrence": 7},
  {"id": "add-mutation-gate", "title": "Add a mutation-kill gate to the audit phase", "files": ["go/internal/audit"], "evidence": "audit-report.md#D2"}
]` + "\n```\n\n" +
	"## Out of scope\n\n- unrelated\n"

// readInboxIDs returns the "id" of every *.json item directly under dir
// (non-recursive), keyed by filename, so a test can assert exactly which
// actions were filed.
func readInboxIDs(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("read inbox dir %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", e.Name(), err)
		}
		id, _ := m["id"].(string)
		out[e.Name()] = id
	}
	return out
}

// writeInboxItem writes a minimal open/processed inbox item carrying id.
func writeInboxItem(t *testing.T, path, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	body, _ := json.Marshal(map[string]any{"id": id, "weight": 0.5})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestParsePreventiveActions_FromRetroReport — AC1 (parse half): the structured
// preventive_actions block is lifted verbatim into typed actions.
func TestParsePreventiveActions_FromRetroReport(t *testing.T) {
	actions, err := ParsePreventiveActions([]byte(fixtureRetro))
	if err != nil {
		t.Fatalf("ParsePreventiveActions: unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("parsed %d actions, want 2", len(actions))
	}
	if actions[0].ID != "builder-task-binding-topn-gate" {
		t.Errorf("actions[0].ID = %q, want builder-task-binding-topn-gate", actions[0].ID)
	}
	if actions[0].WeightHint != 0.92 {
		t.Errorf("actions[0].WeightHint = %v, want 0.92", actions[0].WeightHint)
	}
	if actions[0].Recurrence != 7 {
		t.Errorf("actions[0].Recurrence = %d, want 7", actions[0].Recurrence)
	}
	if actions[1].ID != "add-mutation-gate" {
		t.Errorf("actions[1].ID = %q, want add-mutation-gate", actions[1].ID)
	}
}

// TestParsePreventiveActions_AbsentReturnsNil — edge/OOD: a retro with no
// preventive_actions block yields no actions and no error (the injector must
// no-op, not crash, on the common no-recommendations case).
func TestParsePreventiveActions_AbsentReturnsNil(t *testing.T) {
	actions, err := ParsePreventiveActions([]byte("# Cycle 1 Retrospective\n\n## Out of scope\n- none\n"))
	if err != nil {
		t.Fatalf("unexpected error on absent block: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("parsed %d actions, want 0 for a report with no preventive_actions block", len(actions))
	}
}

// TestFileActions_EndToEndFromFixtureRetro — AC1 (full): parse a fixture retro,
// file its actions, and assert one auto-retro-<cycle>-<slug>.json inbox item
// per action, each carrying the action id.
func TestFileActions_EndToEndFromFixtureRetro(t *testing.T) {
	inbox := t.TempDir()
	actions, err := ParsePreventiveActions([]byte(fixtureRetro))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	written, err := FileActions(inbox, 640, actions, 0.75, fixedNow)
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("wrote %d items, want 2", len(written))
	}
	got := readInboxIDs(t, inbox)
	ids := map[string]bool{}
	for fname, id := range got {
		ids[id] = true
		// Filename must be the documented auto-retro-<cycle>-<slug>.json form.
		if len(fname) < len("auto-retro-640-") || fname[:len("auto-retro-640-")] != "auto-retro-640-" {
			t.Errorf("filed item %q does not use the auto-retro-<cycle>-<slug>.json convention", fname)
		}
	}
	if !ids["builder-task-binding-topn-gate"] || !ids["add-mutation-gate"] {
		t.Errorf("filed ids = %v, want both fixture actions", ids)
	}
}

// TestFileActions_DedupSkipsExistingOpenItem — AC2 (open): an action whose id
// already exists as an OPEN inbox item is skipped (no duplicate filed).
func TestFileActions_DedupSkipsExistingOpenItem(t *testing.T) {
	inbox := t.TempDir()
	writeInboxItem(t, filepath.Join(inbox, "2026-07-06-builder-task-binding-topn-gate.json"), "builder-task-binding-topn-gate")

	actions := []PreventiveAction{{ID: "builder-task-binding-topn-gate", Title: "dup"}}
	written, err := FileActions(inbox, 640, actions, 0.75, fixedNow)
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("wrote %d items, want 0 — an open item with the same id must suppress the re-file", len(written))
	}
	// No auto-retro file should have been created.
	if _, err := os.Stat(filepath.Join(inbox, "auto-retro-640-builder-task-binding-topn-gate.json")); !os.IsNotExist(err) {
		t.Errorf("auto-retro item was filed despite an existing open item (dedup failed)")
	}
}

// TestFileActions_DedupSkipsExistingProcessedItem — AC2 (processed): an action
// whose id was already consumed (lives under inbox/processed/**) is skipped, so
// a recurrence does not re-file a completed fix.
func TestFileActions_DedupSkipsExistingProcessedItem(t *testing.T) {
	inbox := t.TempDir()
	writeInboxItem(t, filepath.Join(inbox, "processed", "cycle-646", "builder-task-binding-topn-gate.json"), "builder-task-binding-topn-gate")

	actions := []PreventiveAction{{ID: "builder-task-binding-topn-gate", Title: "dup"}}
	written, err := FileActions(inbox, 640, actions, 0.75, fixedNow)
	if err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("wrote %d items, want 0 — a processed item with the same id must suppress the re-file", len(written))
	}
}

// TestFileActions_DedupAcrossTwoConsecutiveFailsFilesOnce — AC2 (regression):
// the exact scenario the item names — the same preventive action emitted by two
// consecutive FAIL cycles files exactly once while the first remains open.
func TestFileActions_DedupAcrossTwoConsecutiveFailsFilesOnce(t *testing.T) {
	inbox := t.TempDir()
	actions := []PreventiveAction{{ID: "recurring-fix", Title: "recurring", WeightHint: 0.9, Recurrence: 2}}

	first, err := FileActions(inbox, 640, actions, 0.75, fixedNow)
	if err != nil {
		t.Fatalf("first FileActions: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first file wrote %d, want 1", len(first))
	}
	second, err := FileActions(inbox, 641, actions, 0.75, fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("second FileActions: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second consecutive FAIL wrote %d items, want 0 — dedup must hold while the first is open", len(second))
	}
	filed := readInboxIDs(t, inbox)
	count := 0
	for _, id := range filed {
		if id == "recurring-fix" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("recurring-fix filed %d times across two FAILs, want exactly 1", count)
	}
}

// TestFileActions_UsesDefaultWeightWhenNoHint — AC3 (default): an action with no
// WeightHint is filed at the caller-supplied policy default weight.
func TestFileActions_UsesDefaultWeightWhenNoHint(t *testing.T) {
	inbox := t.TempDir()
	actions := []PreventiveAction{{ID: "plain-action", Title: "no hint"}}
	if _, err := FileActions(inbox, 640, actions, 0.75, fixedNow); err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	w := readItemWeight(t, filepath.Join(inbox, "auto-retro-640-plain-action.json"))
	if w != 0.75 {
		t.Errorf("filed weight = %v, want 0.75 (the policy default)", w)
	}
}

// TestFileActions_UsesHintForRecurrenceFlagged — AC3 (hint): a recurrence-flagged
// action carries its higher WeightHint instead of the default.
func TestFileActions_UsesHintForRecurrenceFlagged(t *testing.T) {
	inbox := t.TempDir()
	actions := []PreventiveAction{{ID: "hot-action", Title: "recurring", WeightHint: 0.95, Recurrence: 5}}
	if _, err := FileActions(inbox, 640, actions, 0.75, fixedNow); err != nil {
		t.Fatalf("FileActions: %v", err)
	}
	w := readItemWeight(t, filepath.Join(inbox, "auto-retro-640-hot-action.json"))
	if w != 0.95 {
		t.Errorf("filed weight = %v, want 0.95 (the recurrence hint, not the 0.75 default)", w)
	}
}

// readItemWeight returns the numeric "weight" field of a filed inbox item.
func readItemWeight(t *testing.T, path string) float64 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read filed item %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	w, ok := m["weight"].(float64)
	if !ok {
		t.Fatalf("filed item %s has no numeric weight field: %v", path, m["weight"])
	}
	return w
}
