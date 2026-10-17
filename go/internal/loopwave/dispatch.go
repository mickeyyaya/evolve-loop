package loopwave

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const RepairWidth = 1

type Launcher interface {
	Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result
}

type PlanFn func(ctx context.Context, wave int) (decisionJSON []byte, cardPackages []string, err error)

type Step string

const (
	StepPreflight Step = "preflight"
	StepPlan      Step = "plan"
	StepAdapt     Step = "adapt"
)

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

type DispatchRequest struct {
	Config    policy.FleetConfig
	Wave      int
	Preflight func() error
	Plan      PlanFn
	Launcher  Launcher
	Routed    fleet.RoutedFn
}

type Outcome struct {
	Ran     bool
	Specs   []fleet.CycleSpec
	Results []fleet.Result
}

type planObserver func(wave, count int, cfg policy.OrchestratorConfig, todos []fleet.Todo, specs []fleet.CycleSpec) func()

func dispatch(ctx context.Context, req DispatchRequest, count int, observe planObserver) (Outcome, error) {
	if err := req.Preflight(); err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPreflight, Err: err}
	}
	decisionJSON, cardPackages, err := req.Plan(ctx, req.Wave)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepPlan, Err: err}
	}
	todos, _, err := fleet.TodosFromTriage(decisionJSON, cardPackages, req.Routed)
	if err != nil {
		return Outcome{}, &StepError{Wave: req.Wave, Step: StepAdapt, Err: err}
	}
	specs, _ := fleet.PlanCycles(todos, count)
	if len(specs) == 0 {
		return Outcome{}, nil
	}
	join := func() {}
	if observe != nil {
		join = observe(req.Wave, count, req.Config.Orchestrator, todos, specs)
	}
	results := req.Launcher.Run(ctx, specs)
	join()
	if len(results) == 0 {
		return Outcome{}, nil
	}
	return Outcome{Ran: true, Specs: specs, Results: results}, nil
}

func (e *Engine) Dispatch(ctx context.Context, req DispatchRequest) (Outcome, error) {
	if !ShouldRunWave(req.Config) {
		return Outcome{}, nil
	}
	out, err := dispatch(ctx, req, req.Config.Count, e.shadowAdmission)
	if err != nil {
		e.warn("Engine.Dispatch", req.Wave, CodeWaveDispatchFailed,
			fmt.Sprintf("wave %d dispatch failed, falling back to sequential: %v", req.Wave, err), stepFields(err, "wave"))
	}
	return out, err
}

func (e *Engine) ForceOneLane(ctx context.Context, req DispatchRequest) (Outcome, error) {
	return dispatch(ctx, req, RepairWidth, nil)
}

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
		t := Tally(out.Results)
		e.warn("Engine.RepairMinWidth", req.Wave, CodeMinWidthRepair,
			fmt.Sprintf("wave %d: min-width repair dispatched %d/%d isolated lane (fleet.count=%d shrank to %d)%s",
				req.Wave, t.OK, t.Total(), fleetCfg.Count, waveCfg.Count, t.DeferredSuffix()), width)
		return true
	default:
		width["cause"] = "empty_backlog"
		e.warn("Engine.RepairMinWidth", req.Wave, CodeWaveEmptyPlan,
			fmt.Sprintf("wave %d planned zero lanes (empty backlog), falling back to sequential", req.Wave), width)
		return false
	}
}

func stepFields(err error, path string) map[string]string {
	fields := map[string]string{"path": path, "error": err.Error()}
	var se *StepError
	if errors.As(err, &se) {
		fields["step"], fields["error"] = string(se.Step), se.Err.Error()
	}
	return fields
}

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

func (e *Engine) routedBase() fleet.RoutedFn {
	return inboxbatch.RoutedResolver(e.lifecycle().InboxDir, e.ports.Protected)
}

func Preflight(projectRoot string) func() error {
	return func() error { return fleet.PreflightControlPlane(projectRoot) }
}
