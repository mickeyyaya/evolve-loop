//go:build acs

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

var evalSlugs = []string{
	"merge-rung0-composition-verdict-entry",
	"merge-rung0-trivial-rebase-fastpath",
	"merge-rung0-composed-tree-gates",
}

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

func TestC786_001_trivial_rebase_carries_audit_forward(t *testing.T) {
	runGoTest(t, shipPkg, "^TestTrivialRebase_CarriesAuditForward$",
		[]string{"TestTrivialRebase_CarriesAuditForward"})
}

func TestC786_002_drift_and_failed_gates_fall_back_to_reaudit(t *testing.T) {
	runGoTest(t, shipPkg,
		"^TestTrivialRebase_(PatchIdDriftFallsBackToReaudit|FailedComposedGatesRejected)$",
		[]string{
			"TestTrivialRebase_PatchIdDriftFallsBackToReaudit",
			"TestTrivialRebase_FailedComposedGatesRejected",
		})
}

func TestC786_003_ledger_verify_kernel_recomputes_patch_id(t *testing.T) {
	runGoTest(t, cmdPkg, "^TestCompositionVerdict_",
		[]string{
			"TestCompositionVerdict_KernelRecomputesPatchId",
			"TestCompositionVerdict_ValidEntryVerifies",
		})
}

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
