//go:build acs

package cycle1591

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	retroPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"

	staleRecord = ".evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json"
)

func runNamedTest(t *testing.T, pkg, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

func checkEval(t *testing.T, slug string, minCommands int) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", slug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", path, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", path, res.Overall)
	}
	if len(res.Commands) < minCommands {
		t.Fatalf("eval %s classified only %d command(s), want >= %d — a vacuous eval is not a PASS",
			path, len(res.Commands), minCommands)
	}
}

func TestC1591_001_removal_claim_asks_the_git_index(t *testing.T) {
	runNamedTest(t, corePkg, "TestRemovalClaimFailures_TrackedButAbsentFromDisk")
}

func TestC1591_002_honest_and_failopen_removals_preserved(t *testing.T) {
	runNamedTest(t, corePkg, "TestRemovalClaimFailures_UntrackedAbsent_StaysHonest")
}

func TestC1591_003_stale_inbox_record_retired_as_tracked_deletion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", "--", staleRecord)
	if err == nil && code == 0 {
		t.Errorf("%s is STILL in the Git index — a filesystem-only retirement is undone by the next "+
			"fresh checkout (retire it with `git rm`, not a delete or a move into .evolve/inbox/processed/)", staleRecord)
	}
	if _, serr := os.Lstat(filepath.Join(root, staleRecord)); serr == nil {
		t.Errorf("%s is still present on disk — the record is still live for the queue scanner", staleRecord)
	}
}

func TestC1591_004_retire_eval_passes_quality_check(t *testing.T) {
	checkEval(t, "retire-stale-retro-prompt-delivery-stall", 3)
}

func TestC1591_005_bridge_submit_wedged_error_is_classified_by_the_consumer(t *testing.T) {
	runNamedTest(t, bridgePkg, "TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier")
}

func TestC1591_006_generic_silence_timeout_is_not_a_delivery_failure(t *testing.T) {
	runNamedTest(t, bridgePkg, "TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause")
}

func TestC1591_007_retro_relaunches_once_on_classified_delivery_failure(t *testing.T) {
	runNamedTest(t, retroPkg, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
	runNamedTest(t, retroPkg, "TestRun_GenericArtifactTimeout_DoesNotRelaunch")
}

func TestC1591_008_format_binding_eval_passes_quality_check(t *testing.T) {
	checkEval(t, "retro-delivery-format-binding", 3)
}
