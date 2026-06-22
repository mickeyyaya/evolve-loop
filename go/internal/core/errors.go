package core

import "errors"

// Sentinel errors. All wrapping in this codebase goes through fmt.Errorf
// with %w so callers can recover the root via errors.Is. Distinct values
// are intentional — every branch on err in the orchestrator checks one
// of these, so collisions would silently mask bugs.
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
	// driver returns ExitArtifactTimeout (81) — the agent's contracted
	// artifact never appeared within the wait window. It lives on the
	// Bridge port (not the concrete bridge adapter) so the generic phase
	// runner can errors.Is-match it WITHOUT importing a specific driver:
	// an OPTIONAL phase that hits this degrades to WARN+advance instead of
	// aborting the whole cycle (Workstream D — cycle-120 build-planner).
	ErrArtifactTimeout = errors.New("core: bridge artifact timeout")

	// ErrTransientBridgeFailure is wrapped into the Bridge.Launch error when a
	// driver returns exit 80, 85, or 86, which represent transient infra issues.
	ErrTransientBridgeFailure = errors.New("core: transient bridge failure")

	// ErrPhaseInvalid means the supplied Phase value isn't a member of
	// the enum.
	ErrPhaseInvalid = errors.New("core: invalid phase")

	// ErrTransitionInvalid means the supplied (from, verdict) pair has
	// no defined successor in the state machine.
	ErrTransitionInvalid = errors.New("core: invalid phase transition")

	// ErrUnsafeConfig means the loaded transition config (legality graph, gates,
	// verdict branches) violates a safety invariant — a flow that could ship
	// without the integrity floor. The composition root computes the violations
	// via ValidateSafetyInvariants at construction; RunCycle/RunCycleFromPhase
	// fail closed with this before any phase runs (PA-DDK DDK-5, ADR-0060 §1a).
	ErrUnsafeConfig = errors.New("core: unsafe transition config")
)

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
