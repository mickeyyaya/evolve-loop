//go:build acs

package cycle1053

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	budgethistoryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	fleetbudgetPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/fleetbudget"
	evolveCmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runPkgTests(t *testing.T, pkg, pattern string, wantPass ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not report PASS (renamed, skipped, or not run):\n%s", name, stdout)
		}
	}
}

func TestC1053_001_collect_reports_median_tokens_per_cycle(t *testing.T) {
	runPkgTests(t, budgethistoryPkg,
		"^TestCollect_MedianTokensPerCycle$",
		"TestCollect_MedianTokensPerCycle")
}

func TestC1053_002_token_median_absent_evidence_never_fabricated(t *testing.T) {
	runPkgTests(t, budgethistoryPkg,
		"^TestCollect_(LegacyTimingWithoutTokensYieldsZeroMedian|NoEvidenceYieldsZeroTokenMedian)$",
		"TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian",
		"TestCollect_NoEvidenceYieldsZeroTokenMedian")
}

func TestC1053_003_shadow_join_pairs_tightest_quota_with_median_tokens(t *testing.T) {
	runPkgTests(t, fleetbudgetPkg,
		"^TestShadowJoin_PairsTightestRemainingWithMedianTokens$",
		"TestShadowJoin_PairsTightestRemainingWithMedianTokens")
}

func TestC1053_004_shadow_join_refuses_half_evidence(t *testing.T) {
	runPkgTests(t, fleetbudgetPkg,
		"^TestShadowJoin_(NoQuotaSignalYieldsNoJoin|NoTokenEvidenceYieldsNoJoin)$",
		"TestShadowJoin_NoQuotaSignalYieldsNoJoin",
		"TestShadowJoin_NoTokenEvidenceYieldsNoJoin")
}

func TestC1053_005_plan_decisions_unchanged_by_shadow_join(t *testing.T) {
	runPkgTests(t, fleetbudgetPkg,
		"^TestPlan_(DecisionUnchangedByShadowJoin|BudgetBranchSizesAffordableLanes|BudgetBranchCapsAtCount|FloorForcedOverspendSetsPaceDelay|TightestWindowBindsAcrossFamilies|FloorFallbackWhenAllUnknown|PaceDelayCappedAtResetHorizon)$",
		"TestPlan_DecisionUnchangedByShadowJoin",
		"TestPlan_BudgetBranchSizesAffordableLanes",
		"TestPlan_BudgetBranchCapsAtCount",
		"TestPlan_FloorForcedOverspendSetsPaceDelay",
		"TestPlan_TightestWindowBindsAcrossFamilies",
		"TestPlan_FloorFallbackWhenAllUnknown",
		"TestPlan_PaceDelayCappedAtResetHorizon")
}

func TestC1053_006_shadow_join_wired_into_composed_wave_path(t *testing.T) {
	runPkgTests(t, evolveCmdPkg,
		"^TestQuotaAwareWaveConfig_(LogsShadowQuotaTokenJoin|EnforceStillJoinsAndDecidesUnchanged|NoTokenEvidenceEmitsNoJoin|NilBudgetEmitsNoJoin)$",
		"TestQuotaAwareWaveConfig_LogsShadowQuotaTokenJoin",
		"TestQuotaAwareWaveConfig_EnforceStillJoinsAndDecidesUnchanged",
		"TestQuotaAwareWaveConfig_NoTokenEvidenceEmitsNoJoin",
		"TestQuotaAwareWaveConfig_NilBudgetEmitsNoJoin")
}
