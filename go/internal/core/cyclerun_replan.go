package core

// cyclerun_replan.go — the ADR-0052 WS2 post-scout re-plan seam.

// postScoutReplanProbe is the WS2-S0 test seam: when non-nil it is invoked each
// time the post-scout re-plan hook fires, so a test can assert the hook's
// call-site contract (fires exactly once per cycle, after scout's handoff is
// recorded, never after build) before any behavior is wired. nil in production.
// Mirrors the PhaseBoundaryCheckpointer package-hook idiom (a DI seam set
// out-of-band, not threaded through every call).
var postScoutReplanProbe func(cr *cycleRun)

// postScoutReplan is the WS2-S0 hook point (ADR-0052): invoked once per cycle
// immediately after scout's handoff has been recorded (CompletedPhases appended
// + cycle-state persisted + phase-boundary checkpoint, all inside recordAndBranch)
// and BEFORE the next selectNext. Firing post-record is precisely what keeps the
// re-plan from widening the run-set or bypassing SpineSatisfiedUpTo — the
// completed scout anchor already exists when it runs. The body is a NO-OP this
// slice; WS2-S3 wires the shadow RePlan behind EVOLVE_ROUTER_REPLAN here.
func (cr *cycleRun) postScoutReplan() {
	if postScoutReplanProbe != nil {
		postScoutReplanProbe(cr)
	}
}
