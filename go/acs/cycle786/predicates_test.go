//go:build acs

// Package cycle786 materializes the cycle-786 acceptance criteria for the
// single fleet-scoped todo merge-rung0-trivial-rebase-carryforward, decomposed
// by scout into three dependency-chained tasks (all triage ## top_n, R9.3):
// composition-verdict ledger entry → trivial-rebase fastpath in
// verifyAuditBinding → composed-tree native gates.
//
// AC map (1:1 with the inbox acceptance[] list):
//
//	AC1 "TestTrivialRebase_CarriesAuditForward (clean rebase, same patch-id:
//	    ship proceeds after native gates, no auditor dispatch)"
//	    → C786_001 runs that ship-package test as a subprocess. RED at
//	      authoring (fast path absent: CodeAuditBindingHeadMoved).
//	AC2 "TestTrivialRebase_PatchIdDriftFallsBackToReaudit"
//	    → C786_002 runs the drift test plus the failed-composed-gates
//	      rejection test (adversarial negatives — pre-existing GREEN today
//	      because moved-HEAD rejects everything; load-bearing the moment the
//	      fast path lands, pinning it never over-accepts).
//	AC3 "TestCompositionVerdict_KernelRecomputesPatchId (tampered entry
//	    rejected); ledger verify covers new entry kind"
//	    → C786_003 runs the cmd/evolve in-process `ledger verify` tests:
//	      tampered entries (drifted diff, forged patch_id) must exit 2, an
//	      honest entry must stay exit 0. RED at authoring (unknown kinds are
//	      ignored by Verify today).
//	AC4 "batch soak: audit dispatches ≈ cycle count; go test -race PASS;
//	    apicover clean"
//	    → manual+checklist in test-report.md (requires a live soaked batch);
//	      the -race half is enforced here by running C786_001/002 under
//	      -race.
//	Step 6b: C786_004 gates the three tasks' .evolve/evals/ files through the
//	    SSOT quality checker (non-vacuous, all commands PASS).
//
// Adversarial axes: negative (C786_002 drift + failed gates; C786_003
// tampered entries), edge (forged-patch-id vs drifted-diff are distinct
// forgeries), semantic (carry-forward, drift fallback, kernel recompute, and
// eval rigor are four distinct behaviors). Every predicate executes the
// system under test (go test subprocess / in-process checker) — no
// source-grep predicates (cycle-85 rule).
package cycle786

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	cmdPkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// evalSlugs are the three triage-committed task slugs (scout-report.md
// ## Selected Tasks); each must ship a .evolve/evals/<slug>.md (Step 6b).
var evalSlugs = []string{
	"merge-rung0-composition-verdict-entry",
	"merge-rung0-trivial-rebase-fastpath",
	"merge-rung0-composed-tree-gates",
}

// runGoTest runs the named tests of pkg (verbose, -race, integration tier)
// and requires an explicit "--- PASS: <name>" for every wantPass — exit 0
// alone never satisfies a predicate (rename/skip gaming).
func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-tags", "integration", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

// AC1: clean rebase + unchanged patch-id + green composed-tree gates →
// verifyAuditBinding accepts the audit+composition chain, no auditor dispatch.
func TestC786_001_trivial_rebase_carries_audit_forward(t *testing.T) {
	runGoTest(t, shipPkg, "^TestTrivialRebase_CarriesAuditForward$",
		[]string{"TestTrivialRebase_CarriesAuditForward"})
}

// AC2 (negatives): patch-id drift falls back to full re-audit; a composition
// entry recording any failed composed-tree gate keeps the fast path closed.
func TestC786_002_drift_and_failed_gates_fall_back_to_reaudit(t *testing.T) {
	runGoTest(t, shipPkg,
		"^TestTrivialRebase_(PatchIdDriftFallsBackToReaudit|FailedComposedGatesRejected)$",
		[]string{
			"TestTrivialRebase_PatchIdDriftFallsBackToReaudit",
			"TestTrivialRebase_FailedComposedGatesRejected",
		})
}

// AC3: `evolve ledger verify` kernel-recomputes composition-verdict patch-ids —
// tampered entries break the chain (exit 2), honest entries stay green.
func TestC786_003_ledger_verify_kernel_recomputes_patch_id(t *testing.T) {
	runGoTest(t, cmdPkg, "^TestCompositionVerdict_",
		[]string{
			"TestCompositionVerdict_KernelRecomputesPatchId",
			"TestCompositionVerdict_ValidEntryVerifies",
		})
}

// Step 6b: each committed task's eval file exists and passes the SSOT quality
// checker with a NON-EMPTY command set (a missing/empty ```bash block would
// PASS vacuously — the existence-check gaming the eval gate forbids).
func TestC786_004_eval_files_pass_quality_check(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, slug := range evalSlugs {
		evalPath := filepath.Join(root, ".evolve", "evals", slug+".md")
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
		if err != nil {
			t.Errorf("eval quality-check %s: %v", evalPath, err)
			continue
		}
		if res.Overall != evalqualitycheck.LevelPass {
			for _, c := range res.Commands {
				if c.Level != evalqualitycheck.LevelPass {
					t.Errorf("eval %s command %q classified level %d: %s", slug, c.Line, c.Level, c.Reason)
				}
			}
			t.Errorf("eval %s overall level %d, want PASS(0)", slug, res.Overall)
			continue
		}
		if len(res.Commands) < 2 {
			t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", slug, len(res.Commands))
		}
	}
}
