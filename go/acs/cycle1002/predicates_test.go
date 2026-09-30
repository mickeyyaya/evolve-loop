//go:build acs

package cycle1002

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runCoreTest(t *testing.T, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, corePkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

func TestC1002_001_evidence_dossier_built_and_written(t *testing.T) {
	runCoreTest(t, "^TestFailureDossier$", "TestFailureDossier")
}

func TestC1002_002_failure_decision_reader_validates_and_falls_back(t *testing.T) {
	runCoreTest(t, "^TestReadFailureDecision$", "TestReadFailureDecision")
}

func TestC1002_003_go_floor_overrides_retry_to_halt(t *testing.T) {
	runCoreTest(t, "^TestDecideAfterRetroFloor_(RoutedRetryOverriddenToHalt|Cycle1001DeterministicHalt|Cycle1001JudgmentHalt)$",
		"TestDecideAfterRetroFloor_RoutedRetryOverriddenToHalt",
		"TestDecideAfterRetroFloor_Cycle1001DeterministicHalt",
		"TestDecideAfterRetroFloor_Cycle1001JudgmentHalt")
}

func TestC1002_004_fallback_byte_identical_regression_guard(t *testing.T) {
	runCoreTest(t, "^TestDecideAfterRetroFloor_FallbackByteIdentical$",
		"TestDecideAfterRetroFloor_FallbackByteIdentical")
}

func TestC1002_005_orchestrator_emit_wiring_present(t *testing.T) {
	root := acsassert.RepoRoot(t)
	agentFile := root + "/agents/evolve-retrospective.md"
	if !acsassert.FileExists(t, agentFile) {
		t.Fatalf("retrospective agent instructions not found at %s", agentFile)
	}
	for _, needle := range []string{
		"failure-decision.json",
		"category", "level", "evidence", "justification", "action", "fix_type",
	} {
		if !acsassert.FileContains(t, agentFile, needle) {
			t.Errorf("retrospective instructions must document the emit contract token %q (inert-API guard)", needle)
		}
	}
	if !acsassert.LineContainsAll(agentFile, "failure-decision.json") {
		t.Error("no line names failure-decision.json — the write-allowlist / output directive was not wired")
	}
}

func TestC1002_006_emitted_decision_shape_round_trips(t *testing.T) {
	runCoreTest(t, "^TestFailureDecisionWiring$", "TestFailureDecisionWiring")
}
