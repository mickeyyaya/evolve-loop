package core

// observer.go — cycle-122 Fix 3: phase-observer auto-spawn from the
// orchestrator's RunCycle (ADR-0030).
//
// Background: pre-v12, the bash dispatcher unconditionally background-
// spawned phase-observer.sh per phase (see ADR-0030 for the silent-
// regression history). The Go port preserved the observer code as a
// manual `evolve phase-observer` subcommand but never re-added the
// orchestrator-side auto-spawn. Cycle-122's tdd-phase hung 10 min
// before the bridge artifact-timeout fired — the observer would have
// emitted a stall_no_output INCIDENT well before that, giving operators
// forensic visibility into "the phase is stuck right now" rather than
// "the phase aborted 10 min ago."
//
// The interface here is intentionally tiny: presence is the contract,
// not shape. The noopObserver default is byte-identical to the pre-fix
// orchestrator behavior, so ObserverPolicy.Autospawn=false + an absent
// WithObserver option together reproduce the pre-ADR-0030 cycle exactly.

import "context"

// Observer is the orchestrator's view of a per-phase stall detector.
// Implementations MUST be safe to call from RunCycle's single-phase
// loop: Start is called once per phase before runner.Run, the returned
// cancel function is called after runner.Run returns (success or
// failure). A nil cancel from Start is allowed; the orchestrator will
// no-op it.
//
// Implementations SHOULD NOT block in Start — return immediately so
// the runner can begin work. Goroutine-spawning implementations
// (e.g., adapters/observer) start their watcher in the background and
// return the cancel that signals it to stop + waits for cleanup.
type Observer interface {
	// Start attaches a stall detector to the about-to-launch phase and
	// returns a cancel function the orchestrator MUST call when the
	// phase finishes. The returned function MUST be idempotent (the
	// orchestrator may call it twice on error paths).
	//
	// phase is the phase identity (e.g., "tdd", "build") — passed
	// separately because PhaseRequest doesn't carry it (the runner
	// derives it from state-machine context). The adapter uses it to
	// resolve the stdout-log path (<workspace>/<agent>-stdout.log).
	Start(ctx context.Context, phase string, req PhaseRequest) (cancel func())
}

// noopObserver is the orchestrator's default when WithObserver was not
// used: Start does nothing, cancel does nothing — byte-identical to
// the pre-ADR-0030 cycle. Kept here (not a separate file) so the
// contract "nil/absent observer implies this exact behavior" lives
// next to the interface that defines it.
type noopObserver struct{}

// Start implements Observer with a permissive no-op default.
func (noopObserver) Start(_ context.Context, _ string, _ PhaseRequest) func() {
	return func() {}
}
