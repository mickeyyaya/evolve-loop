package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func TestHandoffReviewCheck_ReadsTheLedgerOfTheCycleWorkspaceAndTheEvidenceOfTheWorktree(t *testing.T) {
	t.Parallel()
	tree := newHCProject(t)
	req := tree.cycle1950()
	tree.ledger(1950, req, hcMetPredicate(hcCoverAsk), hcMetPredicate(hcGuardAsk))
	workspace := filepath.Join(t.TempDir(), "cycle-1950")
	if err := os.Rename(tree.runDir(1950), workspace); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Cycle: 1950, Phase: string(core.PhaseBuild), Workspace: workspace, Worktree: tree.root, ProjectRoot: t.TempDir()}

	got := handoffReviewCheck(in, "tdd-build")

	if got.Outcome != "pass" || len(got.Errors) != 0 {
		t.Fatalf("check = %+v; want pass on the ledger of the workspace and the predicates of the worktree", got)
	}
}

func TestHandoffReviewCheck_NamesTheEvidenceThatDidNotResolve(t *testing.T) {
	t.Parallel()
	p := newHCProject(t)
	req := p.cycle1950()
	ghost := hcMet(hcCoverAsk, "predicate", "go/acs/cycle1950/predicates_test.go:TestC1950_999_Ghost")
	p.ledger(1950, req, ghost, handoff.Entry{Ask: hcGuardAsk, Status: handoff.StatusMet})
	in := core.ReviewInput{Cycle: 1950, Phase: string(core.PhaseBuild), Workspace: p.runDir(1950), ProjectRoot: p.root}

	got := handoffReviewCheck(in, "tdd-build")

	if got.Outcome != "retry" || strings.Join(got.Unmet, ",") != hcCoverAsk+","+hcGuardAsk {
		t.Fatalf("check = %+v; want retry on both asks", got)
	}
	failed := strings.Join(got.FailedEvidence, "\n")
	for _, want := range []string{hcCoverAsk + ": evidence does not resolve", "TestC1950_999_Ghost", hcGuardAsk + ": a met entry cites no evidence"} {
		if !strings.Contains(failed, want) {
			t.Errorf("failed evidence = %q; want %q", got.FailedEvidence, want)
		}
	}
}

func TestHandoffReviewer_IsInTheCycleReviewerChain(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if !wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{}).Orchestrator.HandoffReviewerWired() {
		t.Fatal("the cycle reviewer chain must hold the handoff reviewer")
	}
}

func TestHandoff_AFileThatHoldsOnlyTheCheckRecordIsAnAbsentLedger(t *testing.T) {
	t.Parallel()
	p := newHCProject(t)
	p.cycle1950()
	p.write(".evolve/runs/cycle-1950/handoff-tdd-build.json", `{"schema_version":1,"pair":"tdd-build","check":{"stage":"shadow","outcome":"error"}}`)

	rc, stdout, _ := runHC(t, "show", "--cycle", "1950", "--project-root", p.root, "--json")
	var shown hcShowResult
	decodeOneObject(t, stdout, &shown)
	if rc != 0 || !strings.Contains(stdout, `{"pair":"tdd-build","outcome":"absent"}`) {
		t.Errorf("rc=%d pairs=%v; a check-only file shows absent, not invalid", rc, shown.Pairs)
	}

	rc, stdout, _ = runHC(t, checkArgs(p, 1950, "tdd-build", "--json")...)
	var checked hcResult
	decodeOneObject(t, stdout, &checked)
	if rc != handoffExitIO || checked.Outcome != "error" || !strings.Contains(strings.Join(checked.Errors, " "), "only the check record") {
		t.Errorf("rc=%d result=%+v; check treats a check-only file as a missing ledger", rc, checked)
	}

	stub := handoffReviewCheck(core.ReviewInput{Cycle: 1950, Workspace: p.runDir(1950), ProjectRoot: p.root}, "tdd-build")
	if err := os.Remove(filepath.Join(p.runDir(1950), "handoff-tdd-build.json")); err != nil {
		t.Fatal(err)
	}
	missing := handoffReviewCheck(core.ReviewInput{Cycle: 1950, Workspace: p.runDir(1950), ProjectRoot: p.root}, "tdd-build")
	if stub.Outcome != missing.Outcome {
		t.Errorf("stub outcome %q, missing outcome %q; the reviewer's own record must not change the next round's outcome", stub.Outcome, missing.Outcome)
	}
}
