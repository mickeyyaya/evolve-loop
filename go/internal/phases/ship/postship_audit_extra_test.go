// postship_audit_extra_test.go — covers the promoteInbox promote path
// (triage-decision.json present with IDs) and findLatestAudit edge cases
// (missing ledger, alien/non-auditor lines) not hit by the existing no-op
// and integration tests.
package ship

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPromoteInbox_WithTriageDecision_Promotes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":5}`)
	mustWrite(t, filepath.Join(root, ".evolve", "runs", "cycle-5", "triage-decision.json"),
		`{"top_n":[{"id":"T-1"},{"id":"T-2"}],"skip_shipped":[{"task_id":"T-3"}]}`)
	opts := &Options{ProjectRoot: root, CommitMessage: "x", Stderr: io.Discard}
	res := &RunResult{CommitSHA: "abcdef1234567890"}
	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox errored: %v", err)
	}
	if !containsLog(*res, "inbox lifecycle drain complete for cycle 5") {
		t.Errorf("missing drain-complete log: %v", res.Logs)
	}
}

// TestPromoteInbox_ProjectsDecisionFromReport pins the deterministic fallback
// (ADR-0047): the triage agent emits only triage-report.md, so promoteInbox
// PROJECTS the companion from it — guaranteed present, so promotion runs and the
// projected companion lands on disk for downstream readers.
func TestPromoteInbox_ProjectsDecisionFromReport(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":7}`)
	// No triage-decision.json — only the report the agent actually wrote.
	mustWrite(t, filepath.Join(root, ".evolve", "runs", "cycle-7", "triage-report.md"),
		"# Triage Decision — Cycle 7\n\n## top_n (commit to THIS cycle)\n- my-task-id: do the thing — priority=H\n\n## deferred\n\n## dropped\n")
	opts := &Options{ProjectRoot: root, CommitMessage: "x", Stderr: io.Discard}
	res := &RunResult{CommitSHA: "abcdef1234567890"}
	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox errored: %v", err)
	}
	if !containsLog(*res, "projected triage-decision.json for cycle 7") {
		t.Errorf("expected projection log, got: %v", res.Logs)
	}
	// The projected companion must be persisted.
	if _, err := readStateMap(filepath.Join(root, ".evolve", "runs", "cycle-7", "triage-decision.json")); err != nil {
		t.Errorf("projected companion not written: %v", err)
	}
	if !containsLog(*res, "inbox lifecycle drain complete for cycle 7") {
		t.Errorf("missing drain-complete log: %v", res.Logs)
	}
}

func TestPromoteInbox_TriageDecisionWithNoIDs_NoPromoteLog(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":6}`)
	mustWrite(t, filepath.Join(root, ".evolve", "runs", "cycle-6", "triage-decision.json"),
		`{"top_n":[],"skip_shipped":[]}`)
	opts := &Options{ProjectRoot: root, CommitMessage: "x", Stderr: io.Discard}
	res := &RunResult{}
	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox errored: %v", err)
	}
	if containsLog(*res, "promote complete") {
		t.Errorf("no IDs should produce no promote-complete log: %v", res.Logs)
	}
}

func TestFindLatestAudit_MissingLedger_NoLedgerShipError(t *testing.T) {
	_, err := findLatestAudit(filepath.Join(t.TempDir(), "does-not-exist.jsonl"), "")
	if err == nil {
		t.Fatal("missing ledger must error")
	}
	wantShipErr(t, err, core.CodeAuditBindingNoLedger, core.ShipClassPrecondition, "no ledger")
}

func TestFindLatestAudit_AlienAndNonAuditorLinesSkipped(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		"not valid json at all\n"+
			`{"role":"builder","kind":"agent_subprocess"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","exit_code":0}`+"\n")
	e, err := findLatestAudit(ledger, "")
	if err != nil {
		t.Fatalf("want the auditor entry, got err %v", err)
	}
	if e.Role != "auditor" {
		t.Errorf("want role=auditor, got %q", e.Role)
	}
}
