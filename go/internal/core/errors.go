package core

import "errors"

var (
	ErrPhaseGateFailed = errors.New("core: phase gate denied action")

	ErrLedgerChainBroken = errors.New("core: ledger hash chain broken")

	ErrLockHeld = errors.New("core: project lock held by another process")

	ErrSubprocessNonZero = errors.New("core: subprocess exited non-zero")

	ErrArtifactTimeout = errors.New("core: bridge artifact timeout")

	ErrTransientBridgeFailure = errors.New("core: transient bridge failure")

	ErrAgentDocMissing = errors.New("core: agent persona doc missing")

	ErrAllFamiliesExhausted = errors.New("core: the dispatch chain ended at a quota wall (exit=85)")

	ErrPhaseInvalid = errors.New("core: invalid phase")

	ErrTransitionInvalid = errors.New("core: invalid phase transition")

	ErrUnsafeConfig = errors.New("core: unsafe transition config")
)

func IsInfraTeardownError(err error) bool {
	return errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure)
}

func isArtifactTimeout(err error) bool { return errors.Is(err, ErrArtifactTimeout) }

func IsOptionalSkippableError(err error) bool {
	return IsInfraTeardownError(err) || errors.Is(err, ErrAgentDocMissing)
}

type ErrCycleLevelFailure struct {
	Phase string
	Cause error
}

func (e *ErrCycleLevelFailure) Error() string {
	return "cycle level failure in phase " + e.Phase + ": " + e.Cause.Error()
}

func (e *ErrCycleLevelFailure) Unwrap() error {
	return e.Cause
}
