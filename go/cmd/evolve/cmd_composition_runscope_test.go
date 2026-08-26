package main

// cmd_composition_runscope_test.go — pins the run-scoping of the composition
// snapshot's ledger reader (the third "latest auditor entry" consumer, found
// by the cycle-1571 H3 review sweep). The ledger is host-global across fleet
// worktrees and, since the H3 producer fix, contains auditor entries for FAIL
// verdicts too — so an unscoped "latest" can hand the RUNG 0 carry-forward a
// sibling lane's (or a FAILed) audit as "the audited snapshot". Same contract
// as ship.findLatestAudit: runID set ⇒ exact match or error; runID=="" keeps
// latest-any.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedger(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLatestAuditEntry_RunScoped_RefusesForeignRun(t *testing.T) {
	t.Parallel()
	p := writeLedger(t,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"SIBLING","git_head":"shaForeign","artifact_sha256":"x"}`)
	_, err := latestAuditEntry(p, "MINE")
	if err == nil || !strings.Contains(err.Error(), "foreign-run entries refused") {
		t.Fatalf("foreign-run-only ledger must refuse to bind, got err=%v", err)
	}
}

func TestLatestAuditEntry_RunScoped_BindsOwnRunNotNewestForeign(t *testing.T) {
	t.Parallel()
	p := writeLedger(t,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"MINE","git_head":"shaMine","artifact_sha256":"a"}`,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"SIBLING","git_head":"shaForeign","artifact_sha256":"b"}`)
	e, err := latestAuditEntry(p, "MINE")
	if err != nil {
		t.Fatalf("latestAuditEntry: %v", err)
	}
	if e.GitHEAD != "shaMine" {
		t.Errorf("bound git_head=%q, want this run's shaMine (newest entry is a sibling's)", e.GitHEAD)
	}
}

func TestLatestAuditEntry_NoRunContext_KeepsLatestAny(t *testing.T) {
	t.Parallel()
	p := writeLedger(t,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"A","git_head":"shaOld","artifact_sha256":"a"}`,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"B","git_head":"shaNew","artifact_sha256":"b"}`)
	e, err := latestAuditEntry(p, "")
	if err != nil {
		t.Fatalf("latestAuditEntry: %v", err)
	}
	if e.GitHEAD != "shaNew" {
		t.Errorf("runID=\"\" got git_head=%q, want latest shaNew (standalone behavior unchanged)", e.GitHEAD)
	}
}
