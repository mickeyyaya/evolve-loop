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

// Launcher launches one wave's lane specs and returns their results; *fleet.Supervisor satisfies it.
type Launcher interface {
	Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result
}

// PlanFn produces one wave's triage-decision.json bytes and the committed cards'
// target packages, the fan-out's fallback when floors are absent.
type PlanFn func(ctx context.Context, wave int) (decisionJSON []byte, cardPackages []string, err error)

// Step names the dispatch step that failed.
type Step string

// The three dispatch steps, in order.
const (
	StepPreflight Step = "preflight"
	StepPlan      Step = "plan"
	StepAdapt     Step = "adapt"
)

// StepError is a dispatch failure tagged with the step that failed.
type StepError struct {
	Wave int
	Step Step
	Err  error
}

// Error renders "wave N: <step label>: <cause>".
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

// Unwrap returns the step's cause.
func (e *StepError) Unwrap() error { return e.Err }

// DispatchRequest carries one wave's collaborators; Config.Count gates Dispatch and caps the fan-out.
type DispatchRequest struct {
	Config    policy.FleetConfig
	Wave      int
	Preflight func() error
	Plan      PlanFn
	Launcher  Launcher
	Routed    fleet.RoutedFn
}

// Outcome reports whether lanes launched, and the launched specs and their results.
type Outcome struct {
	Ran     bool
	Specs   []fleet.CycleSpec
	Results []fleet.Result
}

// dispatch is the one body Dispatch and ForceOneLane share. It never emits,
// because each caller owns its own signal.
func dispatch(ctx context.Context, req DispatchRequest, count int) (Outcome, error) {
	if err := req.Preflight(); err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPreflight, Err: err}
	}
	decisionJSON, cardPackages, err := req.Plan(ctx, req.Wave)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPlan, Err: err}
	}
	// The routed resolver logs each refusal as it fires, so the refusal slice is unused.
	specs, _, err := fleet.PlanFromTriage(decisionJSON, cardPackages, count, req.Routed)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepAdapt, Err: err}
	}
	if len(specs) == 0 {
		return Outcome{}, nil
	}
	return Outcome{Ran: true, Specs: specs, Results: req.Launcher.Run(ctx, specs)}, nil
}

// Dispatch runs the wave path when ShouldRunWave allows it, else returns Outcome{}
// untouched; a step failure also emits one LOOP_WAVE_DISPATCH_FAILED.
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

// ForceOneLane dispatches at most one lane without the ShouldRunWave gate. It
// emits nothing, because RepairMinWidth reports.
func (e *Engine) ForceOneLane(ctx context.Context, req DispatchRequest) (Outcome, error) {
	return dispatch(ctx, req, 1)
}

// RepairMinWidth dispatches one isolated lane when the operator asked for a
// fleet (fleetCfg.Count > 1) but the wave launched none, and reports whether it did.
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

// stepFields projects a dispatch error onto the signal's path, step and error fields.
func stepFields(err error, path string) map[string]string {
	fields := map[string]string{"path": path, "error": err.Error()}
	var se *StepError
	if errors.As(err, &se) {
		fields["step"], fields["error"] = string(se.Step), se.Err.Error()
	}
	return fields
}

// RoutedResolver is the plan-time routing gate; a refusal is a plain WARN line
// because the pool scheduler calls it with no Center.
// See ADR-0074.
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

// routedBase is the routing authority the gate and pruneRouted share. Each reads
// the inbox fresh, and the gate backstops an inbox write that lands between the two reads.
func (e *Engine) routedBase() fleet.RoutedFn {
	return inboxbatch.RoutedResolver(filepath.Join(paths.EvolveDirOf(e.roots.ProjectRoot), "inbox"), e.ports.Protected)
}

// Preflight refuses a wave while the main checkout has uncommitted control-plane
// edits: waves ship from the main tree, where such edits kill audit-passed lanes.
func Preflight(projectRoot string) func() error {
	return func() error { return fleet.PreflightControlPlane(projectRoot) }
}
