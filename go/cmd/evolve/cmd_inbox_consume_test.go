package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func withProjectRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
}

// The fixture's kind is "pipeline-repair": kind:"pipeline-defect" matches ZERO
// live items, so gating on it would pass a synthetic fixture and never fire
// in production.
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
