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
