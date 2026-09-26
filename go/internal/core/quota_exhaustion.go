package core

// abortReasonAllFamiliesExhausted prefixes the C1 abort_reason recorded when a
// phase exhausts its retry budget with exit=85 (provider quota) on EVERY
// attempt. cyclehealth.ClassifyOutcome matches this prefix (via the C1 JSON
// record — the cross-package contract) to classify the cycle DEFERRED instead
// of FAILED_EXPLAINED, and the loop stops resumable (rc=5) instead of burning
// the next cycle into the same drained quota (cycle-656 D2).
const abortReasonAllFamiliesExhausted = "all-families-exhausted"

// allFamiliesQuotaExhausted reports whether an exhausted retry budget is the
// all-CLI-families quota-terminal case (cycle-656): every attempt's bridge
// exit code was 85. The bridge alternates families across attempts (cycle-393
// failover), so with PhaseMaxAttempts >= 2 an all-85 sequence means every
// family in the fallback chain was tried and is drained. A single attempt
// proves nothing about the chain, so len < 2 is never terminal — single-family
// 85 with a healthy sibling keeps the existing failover behavior.
//
// Since WS-876 (llmroute.DispatchTiered) the quota-terminal error feeding this
// classification is only reachable after the LOWEST tier in the plan's tier
// fallback chain (Plan.Tiers, floored at the phase's ModelTierEnvelope.Min /
// the universal "balanced" floor) is also exhausted — an all-85 sequence now
// means every family at every allowed tier is drained, not just at the
// initially resolved tier.
func allFamiliesQuotaExhausted(attemptExits []int) bool {
	if len(attemptExits) < 2 {
		return false
	}
	for _, code := range attemptExits {
		if code != 85 {
			return false
		}
	}
	return true
}

// isQuotaWall reports whether one dispatch came back walled: the runner walks its whole family chain
// inside a single Run and returns exit 85 only when every family it may use answered with a wall (the
// chain steps on exit 85 because llmroute's fallback triggers include it), so a correction re-dispatch,
// a remediation re-run or a resume review gate that sees it defers exactly as the first dispatch does.
// One sample suffices here; the first dispatch's two-sample rule (allFamiliesQuotaExhausted) predates the
// tiered chain and is kept until its tests model the chain.
func isQuotaWall(err error) bool {
	return bridgeExitCode(err) == 85
}
