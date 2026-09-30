//go:build acs

package cycle517

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const setupPkg = "github.com/mickeyyaya/evolve-loop/go/internal/setup"

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

func TestC517_001_AdvisorSanitizerAcceptsTop(t *testing.T) {
	out, code := runGoTest(t, "TestSanitizeAdvisorTier", "github.com/mickeyyaya/evolve-loop/go/internal/core")
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("sanitizeAdvisorTier regressed on the \"top\" tier (exit=%d)\n%s", code, out)
	}
}

func TestC517_002_PolicyTierRankClassifiesTop(t *testing.T) {
	out, code := runGoTest(t, "TestTierRank", "github.com/mickeyyaya/evolve-loop/go/internal/policy")
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("policy.TierRank regressed on the \"top\" tier (exit=%d)\n%s", code, out)
	}
}

func TestC517_003_CanonTierRoundTripsTop(t *testing.T) {
	out, code := runGoTest(t, "TestCanonTier_TopPassesThrough", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("canonTier(\"top\") does not round-trip (exit=%d) — tierFromRank must map rank 4 back to \"top\"\n%s", code, out)
	}
}

func TestC517_004_UpBiasReachesTop(t *testing.T) {
	out, code := runGoTest(t, "TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("biasTier(\"up\", ...) cannot reach \"top\" (exit=%d) — its numeric rank cap must extend to 4\n%s", code, out)
	}
}

func TestC517_005_ClampToTopFloorNotEmpty(t *testing.T) {
	out, code := runGoTest(t, "TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("clampTier degenerates a \"top\" floor clamp to \"\" (exit=%d)\n%s", code, out)
	}
}

func TestC517_006_MaxQualityPresetRecommendsTop(t *testing.T) {
	out, code := runGoTest(t, "TestRecommend_MaxQualityBiasesToTop", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("the max-quality preset does not recommend \"top\" end-to-end (exit=%d)\n%s", code, out)
	}
}

func TestC517_007_TierModelsForSurfacesTop(t *testing.T) {
	out, code := runGoTest(t, "TestTierModelsFor_TopResolvesToModelNotTierName", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("tierModelsFor does not surface a \"top\" key (exit=%d) — abstractTiers must include \"top\"\n%s", code, out)
	}
}
