package core

const abortReasonAllFamiliesExhausted = "all-families-exhausted"

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

func isQuotaWall(err error) bool {
	return bridgeExitCode(err) == 85
}

type quotaWall struct{ cause error }

func (q quotaWall) Error() string { return q.cause.Error() }

func (q quotaWall) Unwrap() []error { return []error{ErrAllFamiliesExhausted, q.cause} }
