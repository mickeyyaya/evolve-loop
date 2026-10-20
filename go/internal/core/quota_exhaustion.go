package core

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

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
	return quotaExit(err) == 85
}

func quotaExit(err error) int {
	var cause LaunchCause
	if errors.As(err, &cause) {
		return llmroute.TieringExit(bridgeExitCode(err), cause.Cause)
	}
	return bridgeExitCode(err)
}

type quotaWall struct{ cause error }

func (q quotaWall) Error() string { return q.cause.Error() }

func (q quotaWall) Unwrap() []error { return []error{ErrAllFamiliesExhausted, q.cause} }

type LaunchCause struct {
	Cause string
	Err   error
}

func (e LaunchCause) Error() string { return e.Err.Error() }

func (e LaunchCause) Unwrap() error { return e.Err }
