//go:build acs

package cycle1473

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runShipTests(t *testing.T, runPattern string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	cmd := exec.Command("go", "test", "./internal/phases/ship", "-run", runPattern, "-count=1")
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("could not run `go test ./internal/phases/ship -run %s` in %s: %v", runPattern, goDir, err)
		}
	}
	return string(out), code
}

func TestC1473_001_GitStageDeterministicClassification(t *testing.T) {
	out, code := runShipTests(t, "TestStageFailureClassification")
	if code != 0 {
		t.Errorf("`go test ./internal/phases/ship -run TestStageFailureClassification -count=1` exit=%d, want 0 — deterministic git-stage fatals are not classified from captured git_stderr yet.\n%s", code, out)
	}
}

func TestC1473_002_TransientShapesSurviveClassification(t *testing.T) {
	const pattern = "TestStageFailureClassification/(rc128_index_lock_contention_stays_transient|unrecognised_stderr_degrades_to_transient|empty_stderr_degrades_to_transient|go_error_text_alone_does_not_classify)"
	out, code := runShipTests(t, pattern)
	if code != 0 {
		t.Errorf("transient-preservation subtests exit=%d, want 0 — index-lock contention, unknown shapes, and Go-composed error text must NOT be reclassified.\n%s", code, out)
	}
	if strings.Contains(out, "warning: no tests to run") || strings.Contains(out, "[no tests to run]") {
		t.Errorf("the -run pattern matched no subtests — this predicate proved nothing:\n%s", out)
	}
}

func TestC1473_003_TwoStrikesRouterSurvives(t *testing.T) {
	out, code := runShipTests(t, "TestStageRefusal|TestStageFailureClassification_TwoStrikesStillApplies")
	if code != 0 {
		t.Errorf("two-strikes router tests exit=%d, want 0 — the stderr classifier must not replace the cycle-1440 refusal memo.\n%s", code, out)
	}
}
