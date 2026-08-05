package main

// cmd_inbox_consume_test.go — RED contract for the operator-facing half of
// Defect A (fault-localization-report.md E4/E5/E8).
//
// The incident's P0 reached .evolve/inbox/consumed/ by an operator `mv`, and
// the ack never rode along because it is a SEPARATE manual command
// (`evolve inbox ack-fingerprint`) that nobody remembered to run. `evolve
// inbox consume <item-path>` makes the move and the ack one transaction, so
// the toil-and-tamper surface the sanctioned flows exist to avoid disappears.
//
// This is the ergonomic seam. The load-bearing self-heal is the reconciler
// in cmd_loop_blockerbreaker_reconcile_test.go, which covers items consumed
// by any route (including a bare `mv`) and repairs the CURRENT live tree.
// Both must share ONE extraction path — do not duplicate the
// unmarshal+parse+append sequence per call site.
//
// Every predicate here drives the REAL production entrypoint (runInbox),
// never a helper in isolation: a wiring proof is a reachability test.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePendingItem drops one JSON item into .evolve/inbox/ (pending) and
// returns its path.
func writePendingItem(t *testing.T, evolveDir, name, body string) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// withProjectRoot points the command's root resolution (envOrCwd
// "EVOLVE_PROJECT_ROOT") at the test tree.
func withProjectRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

// TestRunInbox_Consume_MovesItemAndAcksFingerprint is the transaction: one
// invocation must both land the item in consumed/ AND write the ack ledger,
// with no separate `ack-fingerprint` call.
//
// The fixture's kind is "pipeline-repair" — the value the incident's own P0
// and driving item carry. kind:"pipeline-defect" matches ZERO live items, so
// an implementation gated on it would pass a synthetic fixture and never
// fire in production.
func TestRunInbox_Consume_MovesItemAndAcksFingerprint(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	name := "2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json"
	itemPath := writePendingItem(t, evolveDir, name,
		`{"id":"pipeline-blocker","kind":"pipeline-repair","consumed_by":"`+realConsumedByNarrative+`"}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	if _, err := os.Stat(itemPath); err == nil {
		t.Error("the item must LEAVE the pending inbox — a consume that copies leaves the item drawable by a lane")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "consumed", name)); err != nil {
		t.Fatalf("the item must land in .evolve/inbox/consumed/: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("consumption must ack the fingerprint in the SAME transaction, with no manual `evolve inbox ack-fingerprint` step: %v", err)
	}
	if !strings.Contains(string(raw), incidentFingerprint) {
		t.Fatalf("ledger must carry the consumed item's fingerprint, got %s", raw)
	}
}

// TestRunInbox_Consume_ItemWithoutFingerprintStillMoves pins the gate as
// parse-success: a routine item carrying no fingerprint consumes normally
// and simply writes no ledger record. Non-defect items no-op naturally —
// that is why no `kind` vocabulary is needed, and why none can drift.
func TestRunInbox_Consume_ItemWithoutFingerprintStillMoves(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	itemPath := writePendingItem(t, evolveDir, "plain-feature.json",
		`{"id":"plain-feature","kind":"feature","consumed_by":"console: shipped in #415"}`)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume", itemPath}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("an item with no fingerprint is a normal consumption, not an error: rc=%d stderr=%q", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "consumed", "plain-feature.json")); err != nil {
		t.Fatalf("the item must still land in consumed/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "resolved-fingerprints.json")); err == nil {
		t.Error("no fingerprint parsed ⇒ no ledger record; the ledger must never accumulate empty/garbage entries")
	}
}

// TestRunInbox_Consume_MissingItemReturnsNonZero (negative): a bad path
// fails loudly and touches nothing.
func TestRunInbox_Consume_MissingItemReturnsNonZero(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"consume", filepath.Join(evolveDir, "inbox", "ghost.json")}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatal("a missing item path must exit non-zero, never silently succeed")
	}
	// The failure must be about the ITEM, not about an unrecognised
	// subcommand — otherwise this predicate passes on a tree where `consume`
	// was never registered at all.
	if !strings.Contains(stderr.String(), "ghost.json") {
		t.Errorf("the error must name the item that could not be read, not fall through to subcommand usage; stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "resolved-fingerprints.json")); err == nil {
		t.Error("a failed consume must write no ledger record")
	}
}

// TestRunInbox_Consume_NoArgReturnsUsage (negative/edge): the subcommand is
// registered and reports usage rather than panicking on an empty arg list.
func TestRunInbox_Consume_NoArgReturnsUsage(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)

	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"consume"}, nil, &stdout, &stderr); rc == 0 {
		t.Fatal("`evolve inbox consume` with no item path must exit non-zero")
	}
	if !strings.Contains(stderr.String(), "consume") {
		t.Errorf("usage must name the subcommand; stderr=%q", stderr.String())
	}
}
