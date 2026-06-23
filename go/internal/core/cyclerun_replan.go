package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// cyclerun_replan.go — the ADR-0052 WS2 post-scout re-plan.

// rePlanner is the OPTIONAL re-invokable extension of router.Planner: a planner
// that can produce a SECOND, post-scout plan from measured signals (ADR-0052
// WS1-S3 RePlan). The composition root's PhaseAdvisor implements it; the re-plan
// type-asserts the wired planner to it and no-ops when the planner is not
// re-invokable (fail-safe to the initial plan). Kept separate from router.Planner
// so a non-re-invokable planner (e.g. a scripted test proposer) need not grow a
// RePlan method.
type rePlanner interface {
	RePlan(in router.RouteInput) (*router.PhasePlan, error)
}

// postScoutReplanProbe is the WS2-S0 test seam: when non-nil it is invoked each
// time the post-scout re-plan hook fires, so a test can assert the hook's
// call-site contract (fires exactly once per cycle, after scout's handoff is
// recorded, never after build). nil in production. Mirrors the
// PhaseBoundaryCheckpointer package-hook idiom (a DI seam set out-of-band).
var postScoutReplanProbe func(cr *cycleRun)

// postScoutReplan is the WS2-S0 hook point + WS2-S3 shadow body (ADR-0052):
// invoked once per cycle immediately after scout's handoff has been recorded
// (CompletedPhases appended + cycle-state persisted + phase-boundary checkpoint,
// all inside recordAndBranch) and BEFORE the next selectNext. Firing post-record
// is precisely what keeps the re-plan from widening the run-set or bypassing
// SpineSatisfiedUpTo — the completed scout anchor already exists when it runs.
//
// SHADOW (this slice, EVOLVE_ROUTER_REPLAN=shadow default): the re-plan is
// computed from MEASURED scout signals, clamped to the integrity floor, and
// recorded (phase-replan.json) for soak diffing — but the cycle keeps driving on
// the INITIAL clampedPlan; static still wins. WS2-S6 flips to a swap at
// EVOLVE_ROUTER_REPLAN=advisory. Off ⇒ nothing (byte-identical). Every failure
// path (not re-invokable, no signals, RePlan error) fails safe to the initial plan.
func (cr *cycleRun) postScoutReplan() {
	if postScoutReplanProbe != nil {
		postScoutReplanProbe(cr)
	}
	if cr.o.cfg.RouterReplan == config.StageOff {
		return
	}
	// Same advisor preconditions as the initial plan: DynamicLLM + Advisory.
	if cr.o.cfg.Stage < config.StageAdvisory || cr.o.cfg.Mode != config.ModeDynamicLLM {
		return
	}
	planner, ok := cr.o.planner.(rePlanner)
	if !ok {
		return // wired planner is not re-invokable — fail-safe to the initial plan
	}
	// The re-plan exists to MEASURE need: require scout's handoff signals. Absent
	// (a failed/empty scout) ⇒ no re-plan — there is nothing measured to act on.
	signals, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		// Fail-open (like selectNext's digest): a degraded read must not abort the
		// shadow re-plan — log and fall through; Scout.Present below gates the rest.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN post-scout digest degraded (cycle %d): %v\n", cr.cycle, err)
	}
	if !signals.Scout.Present {
		return
	}
	in := cr.o.advisorPlanInput(cr.ctx, string(PhaseScout), signals, cr.req, cr.state, cr.cs, cr.cycle, cr.envSnap, cr.benchedCLIs)

	// WS2-S4/S5: re-plan ONLY on material divergence — a measured insert_when
	// trigger the initial plan missed — and cap the depth so a thrashing signal
	// can't loop. No mismatch ⇒ the plan already covers the measured need (no
	// churn). At the cap, escalate (a recorded marker) instead of re-planning.
	if !router.PlanMismatch(in, cr.clampedPlan) {
		return
	}
	maxDepth := cr.o.cfg.RePlanMaxDepth
	if maxDepth <= 0 {
		maxDepth = 1 // a directly-constructed cfg (tests) gets the safe default
	}
	if cr.replanDepth >= maxDepth {
		fmt.Fprintf(os.Stderr, "[orchestrator] re-plan depth cap (%d) reached at cycle %d — escalating, not re-planning\n", maxDepth, cr.cycle)
		cr.recordReplanEscalation(maxDepth)
		return
	}

	raw, err := planner.RePlan(in)
	if err != nil || raw == nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN post-scout re-plan failed (keeping initial plan): %v\n", err)
		}
		return // fail open: the initial plan stands
	}
	// Increment ONLY on a successful dispatch (we passed the < maxDepth check
	// above): a nil/error re-plan is fail-open and must NOT consume a depth slot.
	cr.replanDepth++
	clamped, clamps := router.ClampPlanToFloorWith(in, raw, cr.o.resolvedShipFloor(), cr.cs.IntentRequired)
	cr.o.recordPhasePlanKind(cr.ctx, cr.cycle, cr.cs, clamped, clamps, "replan")

	// WS2-S6 advisory flip (the one behavior change, opt-in): at
	// EVOLVE_ROUTER_REPLAN=advisory the re-plan REPLACES the drive plan — but only
	// the CLAMPED re-plan. ClampPlanToFloorWith ran just above and re-asserts the
	// integrity floor (ship⇒build∧audit∧tdd) on the re-plan path, so a re-plan can
	// NEVER weaken ship; the clamp is the sole trust boundary (ADR-0052 D1).
	// registerMintedPhases is idempotent — its runner-existence guard skips any
	// phase the stage-1 plan already wired (runners/catalog/routing all gated on
	// that check), so re-minting A while minting B leaves A once and adds B once.
	// Below advisory (shadow, the default) the re-plan is recorded only — static
	// still drives, so nothing flips silently.
	if cr.o.cfg.RouterReplan == config.StageAdvisory {
		cr.clampedPlan = clamped
		cr.o.registerMintedPhases(clamped)
	}
}

// recordReplanEscalation appends a forensic marker (ADR-0052 WS2-S5) when the
// re-plan depth cap is hit: the cycle escalates rather than re-planning again, so
// a persistent mismatch surfaces to the operator/debugger instead of looping.
// Best-effort — a ledger failure WARNs and is swallowed.
func (cr *cycleRun) recordReplanEscalation(maxDepth int) {
	if err := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS: cr.o.now().UTC().Format(time.RFC3339), Cycle: cr.cycle, Role: "orchestrator",
		Kind: "replan_escalation", ExitCode: 0,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN replan_escalation ledger append: %v\n", err)
	}
}
