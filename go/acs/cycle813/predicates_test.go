//go:build acs

package cycle813

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// acs-predicate: config-check
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
