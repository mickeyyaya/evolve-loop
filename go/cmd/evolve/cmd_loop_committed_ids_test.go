package main

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

// TestFailedCycleCommittedIDs_ReadsTopNAndSkipShipped: the committed set is
// the top_n ∪ skip_shipped union, deduped and order-preserving, so a FAIL
// bumps exactly the ids triage committed to.
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

// TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped: an id triage
// explicitly did not commit to must never enter the committed set — it
// accrues no failure_count and cannot be walked toward the ceiling by a
// failure it had no part in.
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

// TestFailedCycleCommittedIDs_AbsentOrCorruptDecisionIsNil: a missing or
// unparseable decision yields nil, which the drain reads as "no committed
// set known" and falls back to the legacy whole-dir behavior. A panic or a
// bogus non-nil set here would corrupt the failure ledger of a cycle that
// crashed before triage even wrote its verdict.
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
