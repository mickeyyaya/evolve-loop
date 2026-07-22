//go:build acs

// Package cycle1053 materialises the cycle-1053 acceptance criteria for the one
// fleet-scoped item pinned to this lane: token-telemetry-s8-fleet-shadow-join
// (budgethistory median tokens/cycle + fleetbudget shadow quota join, zero
// behavior change).
//
// Predicate strategy. Every predicate DELEGATES to a real in-package Go test
// that exercises the system under test — calls budgethistory.Collect against a
// materialised run workspace, calls fleetbudget.ShadowJoin/Plan, or drives the
// composed CLI sizer quotaAwareWaveConfig — never a source-grep of production
// code (the cycle-85 degenerate-predicate ban). Each delegation requires an
// explicit "--- PASS: <Name>" marker, so a bare exit-0 from "no tests to run"
// (test renamed, deleted, or never written) is REJECTED, not silently green.
//
//   - 001/002 pin the budgethistory half: the new MedianTokensPerCycle field and
//     its absent-evidence discipline (legacy tokenless log ⇒ 0, never fabricated).
//   - 003/004 pin the fleetbudget half: the tightest-window↔tokens join and both
//     negative axes (no quota signal / no token evidence ⇒ no join).
//   - 005 is the slice's CRUX: the zero-behavior-change pin. It re-runs the
//     PRE-EXISTING Plan acceptance tests unmodified alongside the new
//     token-blindness pin, so any allocator change is a hard failure here.
//   - 006 is the WIRING proof: the join must be reachable from the composed CLI
//     wave path (quotaAwareWaveConfig), not an inert exported symbol.
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

// runPkgTests runs `go test -count=1 -v -run <pattern> <pkg>` and fails unless
// every named test reports an explicit PASS. RED today: the S8 symbols
// (Throughput.MedianTokensPerCycle, fleetbudget.ShadowJoin) do not exist, so the
// packages fail to COMPILE and the subprocess exits non-zero.
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

// TestC1053_001_collect_reports_median_tokens_per_cycle — AC1 (verbatim RED test
// named in docs/plans/token-telemetry-2026-07.md). budgethistory.Collect must
// roll each cycle's phasetiming token totals into Throughput.MedianTokensPerCycle
// using the same median discipline as the duration path. The delegated test's
// fixtures discriminate the GROSS definition (input+output+cache_read+cache_write)
// from an input+output-only one, so the metric's meaning is pinned, not assumed.
func TestC1053_001_collect_reports_median_tokens_per_cycle(t *testing.T) {
	runPkgTests(t, budgethistoryPkg,
		"^TestCollect_MedianTokensPerCycle$",
		"TestCollect_MedianTokensPerCycle")
}

// TestC1053_002_token_median_absent_evidence_never_fabricated — AC1 negative
// axis. A legacy phase-timing.json with no Tokens field, and a cycle set with no
// readable evidence at all, must both yield MedianTokensPerCycle == 0 while the
// duration estimate is unaffected. Back-filling tokens from cost or duration
// fails this predicate.
func TestC1053_002_token_median_absent_evidence_never_fabricated(t *testing.T) {
	runPkgTests(t, budgethistoryPkg,
		"^TestCollect_(LegacyTimingWithoutTokensYieldsZeroMedian|NoEvidenceYieldsZeroTokenMedian)$",
		"TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian",
		"TestCollect_NoEvidenceYieldsZeroTokenMedian")
}

// TestC1053_003_shadow_join_pairs_tightest_quota_with_median_tokens — AC2. The
// fleetbudget shadow join must pair the BINDING (tightest) quota window — the
// quotastate.TightestRemaining surface named in the task — with the measured
// median tokens/cycle, and carry both numbers in an operator-legible reason.
func TestC1053_003_shadow_join_pairs_tightest_quota_with_median_tokens(t *testing.T) {
	runPkgTests(t, fleetbudgetPkg,
		"^TestShadowJoin_PairsTightestRemainingWithMedianTokens$",
		"TestShadowJoin_PairsTightestRemainingWithMedianTokens")
}

// TestC1053_004_shadow_join_refuses_half_evidence — AC2 negative axis (both
// directions). No healthy probed window, or no measured token median, must yield
// ok=false and a zero-value join. An implementation that reports the
// quotastate min-seed (1.0) as "100% remaining", or joins against a 0-token
// median, fails here.
func TestC1053_004_shadow_join_refuses_half_evidence(t *testing.T) {
	runPkgTests(t, fleetbudgetPkg,
		"^TestShadowJoin_(NoQuotaSignalYieldsNoJoin|NoTokenEvidenceYieldsNoJoin)$",
		"TestShadowJoin_NoQuotaSignalYieldsNoJoin",
		"TestShadowJoin_NoTokenEvidenceYieldsNoJoin")
}

// TestC1053_005_plan_decisions_unchanged_by_shadow_join — AC3, the CRUX. The
// inbox item's load-bearing criterion is "plan.Lanes decisions unchanged": the
// join is observation, never an allocator input. This runs the NEW token-blindness
// pin together with the PRE-EXISTING Plan acceptance tests, which must still pass
// UNMODIFIED (the Builder is forbidden from touching them — doing so to make the
// slice fit would be caught here and in review).
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

// TestC1053_006_shadow_join_wired_into_composed_wave_path — AC5, the WIRING
// proof (operating-policy: a new exported symbol nothing composes is inert). The
// join must be emitted by quotaAwareWaveConfig — the sizer the production
// budgetAwareWaveConfig path calls every wave — on BOTH stages, must stay silent
// when either half of the evidence is absent, and must leave the nil-budget path
// byte-identically silent.
func TestC1053_006_shadow_join_wired_into_composed_wave_path(t *testing.T) {
	runPkgTests(t, evolveCmdPkg,
		"^TestQuotaAwareWaveConfig_(LogsShadowQuotaTokenJoin|EnforceStillJoinsAndDecidesUnchanged|NoTokenEvidenceEmitsNoJoin|NilBudgetEmitsNoJoin)$",
		"TestQuotaAwareWaveConfig_LogsShadowQuotaTokenJoin",
		"TestQuotaAwareWaveConfig_EnforceStillJoinsAndDecidesUnchanged",
		"TestQuotaAwareWaveConfig_NoTokenEvidenceEmitsNoJoin",
		"TestQuotaAwareWaveConfig_NilBudgetEmitsNoJoin")
}
