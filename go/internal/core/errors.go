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
	// driver returns exit 80, 85, 86, or 124 (boot timeout / unknown prompt /
	// respond-loop guard / command-level timeout kill) — transient infra issues —
	// OR when the driver subprocess exits -1 (signal death) while our own context
	// is cancelled (a completion-wait / phase-timeout teardown SIGKILL'd it;
	// cycle-859). 127 (missing binary) is deliberately NOT transient: an absent
	// CLI is an environment defect that must fail loud, recovered only by the
	// exit-code-triggered family fallback.
	ErrTransientBridgeFailure = errors.New("core: transient bridge failure")

	// ErrAllFamiliesExhausted marks the quota-terminal exhaustion case
	// (cycle-656): every retry attempt for a phase returned exit=85, meaning
	// every CLI family in the fallback chain is quota-drained. The dispatch
	// seam writes a quota-likely checkpoint before aborting with this, so the
	// batch stops resumable (`evolve loop --resume`) instead of failing
	// forward into the same wall.
	ErrAllFamiliesExhausted = errors.New("core: all CLI families quota-exhausted (exit=85)")

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

// IsInfraTeardownError reports whether err is a bridge INFRA teardown — an
// artifact-wait timeout (ErrArtifactTimeout, exit 81) OR a transient bridge
// failure (ErrTransientBridgeFailure, exit 80/85/86/124: quota exhaustion,
// liveness-exhaustion, command-timeout kill; and exit -1 under ctx-cancel: a
// context-cancellation SIGKILL of the driver — cycle-859). Both end the SESSION
// without implying the agent failed: it may have written its contracted
// deliverable before the teardown.
// The phase runner uses this as the single-source trigger for reconciling
// against the on-disk deliverable instead of synthesizing FAIL (cycle-254/255
// timeout false-FAIL; cycle-835 quota false-FAIL). Substantive errors
// (launch/boot/safety/cost) are NEITHER sentinel and are intentionally excluded —
// their output is untrustworthy, so those hard-fail without consulting disk.
func IsInfraTeardownError(err error) bool {
	return errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure)
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
