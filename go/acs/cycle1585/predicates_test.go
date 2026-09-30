//go:build acs

package cycle1585

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	evalSlug = "quota-defer-short-circuits-retro"
)

func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, corePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

func TestC1585_001_all_families_exhausted_dispatches_no_retro(t *testing.T) {
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro")
}

func TestC1585_002_deferred_cycle_state_never_says_retro(t *testing.T) {
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState")
}

func TestC1585_003_wrapped_sentinel_matched_and_bookkeeping_preserved(t *testing.T) {
	runCoreTest(t, "TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro")
}

func TestC1585_004_non_quota_failure_still_dispatches_retro_once(t *testing.T) {
	runCoreTest(t, "TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce")
}

func TestC1585_005_eval_file_passes_quality_check(t *testing.T) {
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
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s classified only %d command(s) — a vacuous eval is not a PASS",
			evalPath, len(res.Commands))
	}
}
