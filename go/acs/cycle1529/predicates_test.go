//go:build acs

// Package cycle1529 carries the cycle-1529 ACS predicates.
//
// Task: close-completion-contract-cancel-parity-stale-item — retire the stale
// inbox item `completion-contract-cancel-parity` as not-observed/already-fixed.
// Scout proved the defect it worried about was fixed and test-locked by the
// `withFinalPoll` generalization (go/internal/bridge/completion.go), pinned by
// go/internal/bridge/completion_cancel_parity_test.go. The item's own
// acceptance criteria say "close as not-observed if none", so the cycle's work
// is a doc-only closure — and the two non-regression predicates below exist to
// prove the closure stayed doc-only.
package cycle1529

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	staleItemID   = "completion-contract-cancel-parity"
	staleItemFile = "2026-07-16T10-30-00Z-completion-contract-cancel-parity.json"
)

// gitTracked reports whether rel (repo-relative) is tracked by git at root.
func gitTracked(root, rel string) bool {
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	return err == nil && code == 0
}

// TestC1529_001_StaleInboxItemRetiredFromLiveBacklog drives the PRODUCTION
// consumer of the backlog (inboxbatch.LoadDir — the same loader behind
// `evolve inbox batches` and the triage prompt) and asserts the closed item is
// no longer drawable by any lane. Behavioral: a `closed: true` field bolted on
// while the file stays in .evolve/inbox/ does NOT satisfy this, because
// LoadDir would still return the item.
func TestC1529_001_StaleInboxItemRetiredFromLiveBacklog(t *testing.T) {
	root := acsassert.RepoRoot(t)
	inboxDir := filepath.Join(root, ".evolve", "inbox")

	items, warns, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		t.Fatalf("RED: inboxbatch.LoadDir(%s): %v", inboxDir, err)
	}
	for _, w := range warns {
		t.Logf("inbox load warning: %s", w)
	}
	if len(items) == 0 {
		t.Fatalf("RED: loader returned 0 items for %s — predicate would pass vacuously", inboxDir)
	}
	for _, it := range items {
		if it.ID == staleItemID {
			t.Errorf("RED: %q is still in the live backlog (%d items loaded) — the closure must move the file out of .evolve/inbox/, not annotate it in place", staleItemID, len(items))
		}
	}
	// Path-level corroboration: the live-backlog path must be gone from disk
	// AND from the index (a file deleted on disk but still tracked ships as a
	// no-op; a file untracked but present is re-drawn by the next lane).
	live := filepath.Join(inboxDir, staleItemFile)
	if _, err := os.Stat(live); err == nil {
		t.Errorf("RED: %s still present on disk", live)
	}
	if gitTracked(root, filepath.Join(".evolve", "inbox", staleItemFile)) {
		t.Errorf("RED: .evolve/inbox/%s is still git-tracked at the live path", staleItemFile)
	}
}

// TestC1529_002_ConsumedRecordCitesParityEvidence asserts the retired item
// landed in the repo's existing closure convention (.evolve/inbox/consumed/,
// the corpus reconcileConsumedFingerprints projects) carrying a closure record
// that names WHY it was closed and WHAT proves it. An empty move with no
// `consumed` block, or a record that cites nothing, fails here.
func TestC1529_002_ConsumedRecordCitesParityEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join(".evolve", "inbox", "consumed", staleItemFile)
	abs := filepath.Join(root, rel)

	raw, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("RED: consumed record unreadable at %s: %v", rel, err)
	}
	// Edge axis: the record must still be well-formed JSON the loader can read
	// (a hand-edited trailing comma silently drops an item from every sweep).
	var item struct {
		ID       string `json:"id"`
		Consumed struct {
			At         string `json:"at"`
			Cycle      string `json:"cycle"`
			Resolution string `json:"resolution"`
		} `json:"consumed"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("RED: %s is not valid JSON: %v", rel, err)
	}
	if item.ID != staleItemID {
		t.Errorf("RED: consumed record id = %q, want %q (identity must survive the move)", item.ID, staleItemID)
	}
	if strings.TrimSpace(item.Consumed.At) == "" {
		t.Errorf("RED: consumed.at is empty in %s", rel)
	}
	if item.Consumed.Cycle != "1529" {
		t.Errorf("RED: consumed.cycle = %q, want \"1529\"", item.Consumed.Cycle)
	}
	res := item.Consumed.Resolution
	for _, needle := range []string{
		"completion_cancel_parity_test.go", // the evidence that closes it
		"not-observed",                     // the item's own closure verdict
	} {
		if !strings.Contains(res, needle) {
			t.Errorf("RED: consumed.resolution does not cite %q — got %q", needle, res)
		}
	}
	// cycle-93 lesson: disk presence without tracking ships as nothing.
	if !gitTracked(root, rel) {
		t.Errorf("RED: %s is untracked — it would be dropped at ship", rel)
	}
}

// TestC1529_003_CancelParityTestsRemainGreen re-runs the four tests that are
// the CLOSING EVIDENCE for this item. Closing an item on the strength of a
// test suite is only sound while that suite is green, so this predicate runs
// it. One named package, -run-narrowed (flaky-predicate-shape rules).
func TestC1529_003_CancelParityTestsRemainGreen(t *testing.T) {
	_ = acsassert.RepoRoot(t) // skip cleanly outside a work tree
	const pkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	const run = "TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout|TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout|TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout|TestArtifactDetector_CtxCancelledShortCircuitsDebounce"

	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", run, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: parity suite not green (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	// Anti-vacuity: `-run` matching nothing also exits 0. Require all four.
	for _, name := range []string{
		"TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout",
		"TestTmuxREPL_StdoutContract_CancelAfterIdle_CompletesNotTimeout",
		"TestTmuxREPL_GitContract_CancelAfterEvidenceCommit_CompletesNotTimeout",
		"TestArtifactDetector_CtxCancelledShortCircuitsDebounce",
	} {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("RED: %s did not report PASS — the closure evidence is incomplete", name)
		}
	}
}

// TestC1529_004_ClosureStaysDocOnly is the negative/scope axis. The task is a
// closure, explicitly NOT the hardening the stale item scoped: touching
// go/internal/bridge/*.go this cycle means the lane re-litigated an
// already-fixed defect without a RED test for it. Compares the worktree
// (committed lane work AND uncommitted edits) against main.
func TestC1529_004_ClosureStaysDocOnly(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "--name-only", "main", "--", "go/internal/bridge")
	if err != nil || code != 0 {
		t.Fatalf("RED: git diff vs main failed (code=%d err=%v): %s", code, err, stderr)
	}
	if changed := strings.TrimSpace(stdout); changed != "" {
		t.Errorf("RED: closure task modified bridge production code (must stay doc-only):\n%s", changed)
	}
}
