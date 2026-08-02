package main

// cmd_loop_committed_ids_test.go — cycle-1176, task
// `wave-lane-claim-into-processing` (inbox item wave-lane-task-quarantine-dead).
//
// failedCycleCommittedIDs (cmd_loop.go:1060) is the reader that feeds
// CycleOutcome.CommittedIDs at the production FAIL site (cmd_loop.go:728). It
// is the hinge of the whole quarantine-dead fix: return the wrong set and the
// drain either bumps nothing (ceiling stays unreachable — the original defect)
// or bumps the entire menu (healthy backlog quarantined after N failures of an
// unrelated task). It had no direct coverage; these tests pin its contract.
//
// The `_ =` on the fixture writes is deliberate: t.Fatal on error would be
// noise for a t.TempDir write that cannot realistically fail, and every
// assertion below reads the value back, so a silent write failure still fails
// the test loudly at the assertion.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
)

// writeTriageDecision drops a triage-decision.json into a fake cycle workspace
// and returns the workspace dir.
func writeTriageDecision(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}
	return ws
}

// TestFailedCycleCommittedIDs_ReadsTopNAndSkipShipped — the positive contract:
// the committed set is the top_n ∪ skip_shipped union, deduped and
// order-preserving, so a FAIL bumps exactly the ids triage committed to.
func TestFailedCycleCommittedIDs_ReadsTopNAndSkipShipped(t *testing.T) {
	t.Parallel()
	ws := writeTriageDecision(t, `{
	  "top_n":[{"id":"wave-lane-task-quarantine-dead"},{"id":"wave-planner-pass-scope-prune"}],
	  "skip_shipped":[{"task_id":"workspace-hygiene-s5-wiring-shadow-default"}],
	  "deferred":[{"id":"not-committed-deferred"}],
	  "dropped":[{"id":"not-committed-dropped"}]
	}`)

	got := cycleoutcome.CommittedIDsFor(ws)

	want := []string{
		"wave-lane-task-quarantine-dead",
		"wave-planner-pass-scope-prune",
		"workspace-hygiene-s5-wiring-shadow-default",
	}
	if len(got) != len(want) {
		t.Fatalf("failedCycleCommittedIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("committed[%d] = %q, want %q (union must be order-preserving)", i, got[i], want[i])
		}
	}
}

// TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped — NEGATIVE, menu
// semantics (PR #366). An id triage explicitly did NOT commit to must never
// enter the committed set: it accrues no failure_count and cannot be walked
// toward the S5 ceiling by a failure it had no part in.
func TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped(t *testing.T) {
	t.Parallel()
	ws := writeTriageDecision(t, `{
	  "top_n":[{"id":"committed-one"}],
	  "deferred":[{"id":"menu-deferred"}],
	  "dropped":[{"id":"menu-dropped"}]
	}`)

	for _, id := range cycleoutcome.CommittedIDsFor(ws) {
		if id == "menu-deferred" || id == "menu-dropped" {
			t.Errorf("uncommitted menu id %q leaked into the committed set — a FAIL would bump backlog no phase worked", id)
		}
	}
}

// TestFailedCycleCommittedIDs_AbsentOrCorruptDecisionIsNil — EDGE, fail-open.
// A missing or unparseable decision yields nil, which the drain reads as
// "no committed set known" and falls back to the legacy whole-dir behavior. A
// panic or a bogus non-nil set here would corrupt the failure ledger of a cycle
// that crashed before triage even wrote its verdict.
func TestFailedCycleCommittedIDs_AbsentOrCorruptDecisionIsNil(t *testing.T) {
	t.Parallel()
	if got := cycleoutcome.CommittedIDsFor(t.TempDir()); got != nil {
		t.Errorf("absent triage-decision.json returned %v, want nil", got)
	}
	if got := cycleoutcome.CommittedIDsFor(writeTriageDecision(t, "not json{")); got != nil {
		t.Errorf("corrupt triage-decision.json returned %v, want nil", got)
	}
	if got := cycleoutcome.CommittedIDsFor(writeTriageDecision(t, `{"top_n":[]}`)); got != nil {
		t.Errorf("empty top_n returned %v, want nil", got)
	}
}
