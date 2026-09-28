package core

// abortReasonAllFamiliesExhausted prefixes the abort_reason recorded when a
// phase exhausts its retry budget with exit=85 (provider quota) on every
// attempt. cyclehealth.ClassifyOutcome matches this prefix to classify the
// cycle DEFERRED instead of FAILED_EXPLAINED, so the loop stops resumable
// (rc=5) instead of burning the next cycle into the same drained quota.
const abortReasonAllFamiliesExhausted = "all-families-exhausted"

// allFamiliesQuotaExhausted reports whether an exhausted retry budget is the
// all-CLI-families quota-terminal case: every attempt's bridge exit code was
// 85. The bridge alternates families across attempts, so with
// PhaseMaxAttempts >= 2 an all-85 sequence means every family in the fallback
// chain was tried and is drained. A single attempt proves nothing about the
// chain, so len < 2 is never terminal.
//
// The quota-terminal error feeding this classification is only reachable
// after the lowest tier in the plan's tier fallback chain (floored at the
// phase's ModelTierEnvelope.Min, or the universal "balanced" floor) is also
// exhausted, so an all-85 sequence means every family at every allowed tier
// is drained, not just at the initially resolved tier.
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

// isQuotaWall reports whether one dispatch came back walled: the runner walks
// its whole family chain inside a single Run and returns exit 85 when the walk
// exhausted every family after meeting a wall, so one sample suffices here — unlike
// allFamiliesQuotaExhausted's two-sample rule, which predates the tiered
// chain and is kept until its tests model it.
func isQuotaWall(err error) bool {
	return bridgeExitCode(err) == 85
}

type quotaWall struct{ cause error }

func (q quotaWall) Error() string { return q.cause.Error() }

func (q quotaWall) Unwrap() []error { return []error{ErrAllFamiliesExhausted, q.cause} }
