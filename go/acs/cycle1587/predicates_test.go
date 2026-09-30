//go:build acs

package cycle1587

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	evalSlug = "pipeline-defect-pipeline-blocker-cycle1582"
)

func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+topLevel(name)+"$", corePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			topLevel(name), corePkg, code, err, stdout, stderr)
	}
	marker := "--- PASS: " + name
	if !strings.Contains(stdout, marker) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

func topLevel(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

func TestC1587_001_explained_negative_verdict_stays_coherent(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt")
	runCoreTest(t, "TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt")
}

func TestC1587_002_unexplained_negative_detected_except_verified_latewrite(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_ForgedVerdict_Halts")
	runCoreTest(t, "TestDetectVerdictIncoherence_ReconcileUsesFullVerify")
}

func TestC1587_003_malformed_artifact_never_laundered_into_reconciliation(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_ReconcileUsesFullVerify")
	runCoreTest(t, "TestDetectVerdictIncoherence_WorkspaceReasonFileAlone_StillHalts")
}

func TestC1587_004_deferred_arm_appends_no_failed_record(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_FailedRecord_appended")
}

func TestC1587_005_deferred_arm_queues_no_carryover_todo(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_P0_carryover_todo_queued")
}

func TestC1587_006_deferred_arm_never_force_runs_retro(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/retro_runner_never_invoked_for_learning")
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro")
	runCoreTest(t, "TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro")
}

func TestC1587_007_non_exhaustion_failure_still_learns_normally(t *testing.T) {
	runCoreTest(t, "TestDispatch_SingleFamily85WithSibling_FailureLearningUnchanged")
}

func TestC1587_008_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
}
