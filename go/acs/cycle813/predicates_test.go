//go:build acs

// Package cycle813 materializes the cycle-813 acceptance criteria for this
// fleet lane's sole committed task, verify-fleet-soak-ci-green /
// confirm-integration-tag-in-go-workflow (fix-fleet-soak-red-ci, P0 0.99).
//
// Scout found the original inbox defect (soak test asserting pre-orphan-sweep
// behavior) already fixed on main as of cycle-809: the actual CI red was an
// apicover-enforce gap (go/internal/core.LaneScope/LaneScopeFile,
// go/internal/cyclestate.SkippedPhase), both now covered by
// go/internal/core/lanescope_apicover_test.go and
// go/internal/cyclestate/result_test.go. This cycle's job is verification,
// not re-fix (see scout-report.md "Selected Tasks").
//
// Task 1's AC ("gh run list shows HEAD completed success") is inherently
// non-hermetic — it reads live GitHub Actions state that changes independently
// of this repo's tree and cannot be pinned as a repeatable regression
// predicate. It is dispositioned manual+checklist in test-report.md instead
// (AC-Materialization Contract) rather than gamed with a source-grep stand-in.
//
// Task 2's AC ("go workflow YAML passes -tags=integration on the step that
// runs ./cmd/evolve/...") is a genuine config-presence check: the acceptance
// criterion IS "does this exact configuration line exist", not "does a
// magic string mentioning it exist somewhere". That is the documented
// `// acs-predicate: config-check` waiver case (Predicate Quality section) —
// the sole exception to the FileContains-over-source-is-degenerate rule.
package cycle813

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// acs-predicate: config-check
//
// TestC813_001_GoWorkflowIntegrationTagPresent pins that the CI step covering
// ./cmd/evolve/... (where TestFleetSoak_AllFourInvariants lives, gated
// //go:build integration) actually passes -tags integration. If this line is
// ever dropped, the soak suite silently stops running in CI ("no tests to
// run") and a real regression there would go undetected by the workflow —
// the exact "beyond-the-ask" risk scout flagged this cycle.
func TestC813_001_GoWorkflowIntegrationTagPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	workflow := filepath.Join(root, ".github", "workflows", "go.yml")

	if !acsassert.FileContains(t, workflow, "working-directory: go") {
		t.Fatalf("go.yml no longer scopes the build-test job to the go/ module (working-directory: go missing) — step-path assumption below is stale")
	}
	if !acsassert.FileMatchesRegex(t, workflow, `go test -race -count=1 -tags integration [^\n]*\$\(go list \./\.\.\. `) {
		t.Errorf("go.yml test step no longer runs `go test -race -count=1 -tags integration ...` over ./... — TestFleetSoak_AllFourInvariants (cmd/evolve, //go:build integration) would silently stop executing in CI")
	}
}

// TestC813_002_ApicoverEnforceGapsClosed re-verifies (does not re-derive) that
// the two exported symbols scout identified as the actual root cause of the
// CI-red inbox item — LaneScope/LaneScopeFile (internal/core) and
// SkippedPhase (internal/cyclestate) — are named by a real _test that
// exercises them, not just present in source. Both must already be green on
// HEAD per scout-report.md; a regression here means the apicover-enforce gap
// (warnship_apicover_ci_gap disease) has recurred a fourth time.
func TestC813_002_ApicoverEnforceGapsClosed(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1",
		"-run", "TestLaneScope_ExportedSchemaAndFilename",
		"-v", "../../internal/core/...",
	)
	if err != nil {
		t.Fatalf("failed to run go test for LaneScope apicover regression: %v (stderr: %s)", err, stderr)
	}
	if code != 0 {
		t.Errorf("TestLaneScope_ExportedSchemaAndFilename did not pass (exit %d): stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !contains(stdout, "--- PASS: TestLaneScope_ExportedSchemaAndFilename") {
		t.Errorf("expected explicit --- PASS marker for TestLaneScope_ExportedSchemaAndFilename, got: %s", stdout)
	}

	stdout2, stderr2, code2, err2 := acsassert.SubprocessOutput(
		"go", "test", "-count=1",
		"-run", "TestSkippedPhase",
		"-v", "../../internal/cyclestate/...",
	)
	if err2 != nil {
		t.Fatalf("failed to run go test for SkippedPhase apicover regression: %v (stderr: %s)", err2, stderr2)
	}
	if code2 != 0 {
		t.Errorf("SkippedPhase-covering test did not pass (exit %d): stdout=%s stderr=%s", code2, stdout2, stderr2)
	}
	if !contains(stdout2, "--- PASS") {
		t.Errorf("expected explicit --- PASS marker for a SkippedPhase-covering test, got: %s", stdout2)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || (len(needle) > 0 && indexOf(haystack, needle) >= 0))
}

func indexOf(haystack, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(haystack); i++ {
		if haystack[i:i+n] == needle {
			return i
		}
	}
	return -1
}
