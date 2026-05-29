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
	if !containsLog(*res, "inbox lifecycle promote complete for cycle 5") {
		t.Errorf("missing promote-complete log: %v", res.Logs)
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
	_, err := findLatestAudit(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
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
	e, err := findLatestAudit(ledger)
	if err != nil {
		t.Fatalf("want the auditor entry, got err %v", err)
	}
	if e.Role != "auditor" {
		t.Errorf("want role=auditor, got %q", e.Role)
	}
}
