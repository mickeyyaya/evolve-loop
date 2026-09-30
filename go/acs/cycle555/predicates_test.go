//go:build acs

package cycle555

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ciparityPkg = "github.com/mickeyyaya/evolve-loop/go/internal/ciparity"

func runCiparityTest(t *testing.T, runFilter string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, ciparityPkg)
	return stdout + "\n" + stderr, code
}

func requireRanAndGreen(t *testing.T, out string, code, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — the SSOT's behavioral tests are unwritten or renamed:\n%s", out)
		return
	}
	ran := strings.Count(out, "--- PASS") + strings.Count(out, "--- FAIL")
	if ran < min {
		t.Errorf("only %d test(s) ran, need >= %d (or internal/ciparity failed to build):\n%s", ran, min, out)
		return
	}
	if code != 0 || strings.Contains(out, "--- FAIL") {
		t.Errorf("internal/ciparity SSOT tests failed (exit=%d):\n%s", code, out)
	}
}

func TestC555_001_CoverageTagsIncludeACS(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTags_IncludesACSTag$")
	requireRanAndGreen(t, out, code, 1)
}

func TestC555_002_CoverageArgsThreadBothTags(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTestArgs_ThreadsBothTagsAndPreservesPkgOrder$")
	requireRanAndGreen(t, out, code, 1)
}

func TestC555_003_TagGatedFixtureMeasuredThroughSSOT(t *testing.T) {
	out, code := runCiparityTest(t, "^TestCoverageTestArgs_TagGatedFixtureMeasuresTaggedCoverage$")
	requireRanAndGreen(t, out, code, 1)
}
