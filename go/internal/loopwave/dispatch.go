package loopwave

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// Launcher launches one wave's disjoint lane specs and reports their
// results. *fleet.Supervisor satisfies it — production needs no adapter.
type Launcher interface {
	Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result
}

// PlanFn produces one wave's single-writer triage output: the raw
// triage-decision.json bytes plus the committed cards' target packages (a
// fallback when floors are absent). ctx is the loop's cancellable context —
// plan-path reads observe loop shutdown; wave lets production pick per-wave
// state.
type PlanFn func(ctx context.Context, wave int) (decisionJSON []byte, cardPackages []string, err error)

// Step names the dispatch step that failed.
type Step string

// The three dispatch steps, in order.
const (
	StepPreflight Step = "preflight"
	StepPlan      Step = "plan"
	StepAdapt     Step = "adapt"
)

// StepError is a typed dispatch failure: its Error() renders the exact
// wrapped text the host has always printed ("wave N: <step>: <cause>") and
// Unwrap keeps the cause errors.Is-matchable; the type is what lets the
// producer stamp fields.step.
type StepError struct {
	Wave int
	Step Step
	Err  error
}

func (e *StepError) Error() string {
	switch e.Step {
	case StepPreflight:
		return fmt.Sprintf("wave %d: control-plane preflight: %v", e.Wave, e.Err)
	case StepPlan:
		return fmt.Sprintf("wave %d: triage plan: %v", e.Wave, e.Err)
	default:
		return fmt.Sprintf("wave %d: adapt triage plan: %v", e.Wave, e.Err)
	}
}

func (e *StepError) Unwrap() error { return e.Err }

// DispatchRequest is ONE wave's collaborators: the fleet config (Count gates
// Dispatch and sizes the fan-out), the wave index, the control-plane
// preflight, the plan source, the launcher and the ADR-0074 routed resolver.
type DispatchRequest struct {
	Config    policy.FleetConfig
	Wave      int
	Preflight func() error
	Plan      PlanFn
	Launcher  Launcher
	Routed    fleet.RoutedFn
}

// Outcome is what a dispatch produced: Ran reports lanes launched; Specs and
// Results are the launched lanes and their results.
type Outcome struct {
	Ran     bool
	Specs   []fleet.CycleSpec
	Results []fleet.Result
}

// dispatch is the ONE body the wave fan-out and the min-width repair share:
// preflight → plan → adapt into <= count disjoint lane specs → launch, UNLESS
// the adapted plan is empty (ran=false, the launcher never invoked, so the
// caller falls back to sequential instead of burning an iteration). Every
// failure is a *StepError with ran=false and the launcher never invoked. Pure
// — it never emits; the exported entries own the console sentences.
func dispatch(ctx context.Context, req DispatchRequest, count int) (Outcome, error) {
	if err := req.Preflight(); err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPreflight, Err: err}
	}
	decisionJSON, cardPackages, err := req.Plan(ctx, req.Wave)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPlan, Err: err}
	}
	// ADR-0074 plan-time gate: refusals (console-routed ids) are logged by
	// the routed resolver the moment they fire, so the slice is discarded.
	specs, _, err := fleet.PlanFromTriage(decisionJSON, cardPackages, count, req.Routed)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepAdapt, Err: err}
	}
	if len(specs) == 0 {
		return Outcome{}, nil
	}
	return Outcome{Ran: true, Specs: specs, Results: req.Launcher.Run(ctx, specs)}, nil
}

// Dispatch runs one batch iteration's wave path when ShouldRunWave gates it
// on (Outcome{} with no side effects otherwise, so the caller falls through
// to the sequential body). A step failure returns the *StepError AND emits
// ONE LOOP_WAVE_DISPATCH_FAILED (path=wave) whose reason is the sentence the
// coordinator used to print; the preflight only ever runs on the wave path.
func (e *Engine) Dispatch(ctx context.Context, req DispatchRequest) (Outcome, error) {
	if !ShouldRunWave(req.Config) {
		return Outcome{}, nil
	}
	out, err := dispatch(ctx, req, req.Config.Count)
	if err != nil {
		e.warn("Engine.Dispatch", req.Wave, CodeWaveDispatchFailed,
			fmt.Sprintf("wave %d dispatch failed, falling back to sequential: %v", req.Wave, err), stepFields(err, "wave"))
	}
	return out, err
}

// ForceOneLane is the min-width repair's dispatcher (cycle 547): up to ONE
// disjoint candidate through the same isolated launcher path, capped at a
// single lane, WITHOUT the ShouldRunWave gate (the caller already knows the
// operator wanted a fleet and only reached here because the wave-sized count
// shrank). Silent: RepairMinWidth reports.
func (e *Engine) ForceOneLane(ctx context.Context, req DispatchRequest) (Outcome, error) {
	return dispatch(ctx, req, 1)
}

// RepairMinWidth is the cycle-547 min-width repair the coordinator reaches on
// Dispatch's (ran=false, err=nil) case. Eligibility is the operator-asserted
// width alone: fleetCfg.Count>1 means the operator wanted a fleet, so both
// the quota-shrunk shape (waveCfg.Count<=1) and the empty-plan-at-full-
// capacity shape repair to one isolated lane rather than the leak-prone
// sequential fallthrough; true sequential stays reserved for
// fleetCfg.Count<=1. The four branches each report ONE loop.wave WARN whose
// reason keeps the sentence the inline switch printed: guard not met →
// LOOP_WAVE_EMPTY_PLAN cause=empty_triage_plan (nothing invoked); a lane
// dispatched → LOOP_MIN_WIDTH_REPAIR, handled=true (the caller continues);
// an empty backlog → LOOP_WAVE_EMPTY_PLAN cause=empty_backlog; a step error →
// LOOP_WAVE_DISPATCH_FAILED path=repair.
func (e *Engine) RepairMinWidth(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, req DispatchRequest) (handled bool) {
	width := map[string]string{"desired": strconv.Itoa(fleetCfg.Count), "realized": strconv.Itoa(waveCfg.Count)}
	if fleetCfg.Count <= 1 {
		width["cause"] = "empty_triage_plan"
		e.warn("Engine.RepairMinWidth", req.Wave, CodeWaveEmptyPlan,
			fmt.Sprintf("wave %d planned zero lanes (empty triage plan), falling back to sequential", req.Wave), width)
		return false
	}
	out, err := e.ForceOneLane(ctx, req)
	switch {
	case err != nil:
		e.warn("Engine.RepairMinWidth", req.Wave, CodeWaveDispatchFailed,
			fmt.Sprintf("wave %d min-width repair failed, falling back to sequential: %v", req.Wave, err), stepFields(err, "repair"))
		return false
	case out.Ran:
		e.warn("Engine.RepairMinWidth", req.Wave, CodeMinWidthRepair,
			fmt.Sprintf("wave %d: min-width repair dispatched %d/%d isolated lane (fleet.count=%d shrank to %d)",
				req.Wave, len(out.Results)-FailedLanes(out.Results), len(out.Results), fleetCfg.Count, waveCfg.Count), width)
		return true
	default:
		width["cause"] = "empty_backlog"
		e.warn("Engine.RepairMinWidth", req.Wave, CodeWaveEmptyPlan,
			fmt.Sprintf("wave %d planned zero lanes (empty backlog), falling back to sequential", req.Wave), width)
		return false
	}
}

// stepFields projects a dispatch error onto the triage fields: the path
// (wave | repair), the failing step and the step's own error. Every error the
// two entries see is a *StepError; the fallback names the whole error.
func stepFields(err error, path string) map[string]string {
	fields := map[string]string{"path": path, "error": err.Error()}
	var se *StepError
	if errors.As(err, &se) {
		fields["step"], fields["error"] = string(se.Step), se.Err.Error()
	}
	return fields
}

// RoutedResolver is the composition-root wiring of the ADR-0074 plan-time
// gate: a fresh inbox load per wave (mid-batch inbox changes must be seen),
// the protected-surface port, and a WARN line the moment a console-routed id
// is refused — kept as a line because the pool scheduler reaches it with no
// Center (F6).
func (e *Engine) RoutedResolver() fleet.RoutedFn {
	base := e.routedBase()
	return func(id string) (bool, string) {
		routed, reason := base(id)
		if routed {
			fmt.Fprintf(e.stderr, "[fleet] WARN: plan-time gate refused console-routed item %q (%s) — operator-owned, worked at a batch boundary via manual ship (ADR-0074)\n", id, reason)
		}
		return routed, reason
	}
}

// routedBase is the ONE plan-time routing authority: a fresh inbox load under
// the injected scope predicate. The dispatch gate (RoutedResolver) and the
// plan prune (pruneRouted) both consult it — the same logic, each its own
// fresh read — so the prune drops what the gate would refuse; an inbox write
// landing between the two reads is caught by the gate, the backstop.
func (e *Engine) routedBase() fleet.RoutedFn {
	return inboxbatch.RoutedResolver(filepath.Join(paths.EvolveDirOf(e.roots.ProjectRoot), "inbox"), e.ports.Protected)
}

// Preflight is the S3 dirty-control-plane guard: a closure over
// fleet.PreflightControlPlane against the MAIN checkout (waves ship from the
// main tree, so that is the tree whose uncommitted control-plane edits kill
// audit-PASSED lanes at ship time).
func Preflight(projectRoot string) func() error {
	return func() error { return fleet.PreflightControlPlane(projectRoot) }
}
