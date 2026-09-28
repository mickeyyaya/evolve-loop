package core

import "errors"

var (
	// ErrPhaseGateFailed is returned when a trust-kernel guard denies
	// an action (ship, role-write outside allowlist, etc).
	ErrPhaseGateFailed = errors.New("core: phase gate denied action")

	// ErrLedgerChainBroken is returned when the SHA-chain over
	// .evolve/ledger.jsonl entries cannot be reproduced.
	ErrLedgerChainBroken = errors.New("core: ledger hash chain broken")

	// ErrLockHeld is returned when .evolve/.lock is held by another
	// concurrent runner (multi-project safety).
	ErrLockHeld = errors.New("core: project lock held by another process")

	// ErrSubprocessNonZero is returned when a wrapped subprocess
	// (bridge, sandbox-exec, bwrap) exited non-zero.
	ErrSubprocessNonZero = errors.New("core: subprocess exited non-zero")

	// ErrArtifactTimeout is wrapped into the Bridge.Launch error when a
	// driver returns ExitArtifactTimeout (81): the agent's contracted
	// artifact never appeared within the wait window. 127 (missing binary)
	// is deliberately NOT transient: an absent CLI is an environment defect
	// that must fail loud.
	ErrArtifactTimeout = errors.New("core: bridge artifact timeout")

	// ErrTransientBridgeFailure is wrapped into the Bridge.Launch error when a
	// driver returns exit 80, 85, 86, or 124, or exits -1 while our own
	// context is already cancelled.
	ErrTransientBridgeFailure = errors.New("core: transient bridge failure")

	// ErrAgentDocMissing marks a phase-dispatch failure whose cause is the
	// agent persona doc not existing on disk.
	ErrAgentDocMissing = errors.New("core: agent persona doc missing")

	// ErrAllFamiliesExhausted marks a dispatch whose fallback chain ran out
	// after meeting a quota wall (exit=85): no family completed it, so the
	// cycle defers instead of failing.
	ErrAllFamiliesExhausted = errors.New("core: the dispatch chain ended at a quota wall (exit=85)")

	// ErrPhaseInvalid means the supplied Phase value isn't a member of
	// the enum.
	ErrPhaseInvalid = errors.New("core: invalid phase")

	// ErrTransitionInvalid means the supplied (from, verdict) pair has
	// no defined successor in the state machine.
	ErrTransitionInvalid = errors.New("core: invalid phase transition")

	// ErrUnsafeConfig means the loaded transition config (legality graph, gates,
	// verdict branches) violates a safety invariant — a flow that could ship
	// without the integrity floor.
	//
	// See ADR-0060.
	ErrUnsafeConfig = errors.New("core: unsafe transition config")
)

// IsInfraTeardownError reports whether err is a bridge infra teardown: an
// artifact-wait timeout or a transient bridge failure. Both end the session
// without implying the agent failed.
func IsInfraTeardownError(err error) bool {
	return errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure)
}

// isArtifactTimeout must never widen to the IsInfraTeardownError union: it
// feeds only the failure-diagnostic writer's exit_code/delivery-cause
// attribution, a narrower question than teardown reconciliation.
func isArtifactTimeout(err error) bool { return errors.Is(err, ErrArtifactTimeout) }

// IsOptionalSkippableError is the full admission predicate for
// optionalInfraSkip's error gate: infra teardown or a missing agent persona
// doc.
func IsOptionalSkippableError(err error) bool {
	return IsInfraTeardownError(err) || errors.Is(err, ErrAgentDocMissing)
}

// ErrCycleLevelFailure wraps a phase failure that should escalate to cycle-level
// instead of batch-fatal abort.
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
