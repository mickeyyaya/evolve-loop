package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const ParkReason = "deadline twice"

const releaseReason = "orchestrator: the lane passed its lease deadline and was stopped"

type Claims interface {
	Release(todo, reason string) error
	Park(todo, reason string) error
}

type Admit func(inFlight, pending []fleet.Todo) int

type Pool struct {
	Target      int
	MinLanes    int
	Concurrency int
	MaxLanes    int
	Admit       Admit
	Freshen     func(fleet.CycleSpec) (fleet.CycleSpec, bool)
	Launch      fleet.LaunchFn
	Leases      *Leases
	Claims      Claims
	Reap        func(Lane)
	Publish     Publish
	Halted      func() bool
	Intake      *Intake
	Stage       string
	Window      *Window
	Resupply    func(held []fleet.Todo) []fleet.Todo
}

var transitionCodes = map[fleet.PoolTransitionKind]signalcenter.Code{
	fleet.PoolLaunched:   CodeLaneLaunched,
	fleet.PoolExited:     CodeLaneExited,
	fleet.PoolBackfilled: CodeLaneBackfilled,
	fleet.PoolIdle:       CodePoolIdle,
	fleet.PoolResupplied: CodePoolResupplied,
}

var errIntakeNeedsAdmit = errors.New("orchestrator: a pool with an intake needs an admit function: the freed cells and the limit act through it")

func (p Pool) Run(ctx context.Context, backlog []fleet.Todo) []fleet.Result {
	if p.Intake != nil && p.Admit == nil {
		return refusedResults(len(backlog), errIntakeNeedsAdmit)
	}
	in := p.Intake
	if in == nil {
		in = NewIntake(max(p.Target, 1))
	}
	next := p.Admit
	if next != nil {
		next = in.Gate(next)
	}
	launch := p.Launch
	if p.Leases != nil {
		launch = p.Leases.Watch(p.Launch, p.recover)
	}
	cfg := fleet.PoolConfig{Target: p.Target, Concurrency: p.Concurrency, MaxLaunches: p.MaxLanes,
		Halted: func() bool { return in.Stopped() || (p.Halted != nil && p.Halted()) },
		Next:   next, Kick: in.Wake(), Window: p.admissionWindow(), Resupply: p.Resupply,
		Prepare: func(td fleet.Todo, spec fleet.CycleSpec) (fleet.CycleSpec, bool) {
			in.Hold(td.ID)
			return p.prepare(td, spec)
		}}
	return fleet.RunPool(ctx, cfg, backlog, launch, p.transition)
}

func refusedResults(n int, err error) []fleet.Result {
	results := make([]fleet.Result, n)
	for i := range results {
		results[i] = fleet.Result{Index: i, ExitCode: -1, Err: err}
	}
	return results
}

func (p Pool) admissionWindow() func() int {
	if p.Window == nil || p.Stage != StageEnforce {
		return nil
	}
	return p.Window.Size
}

func (p Pool) prepare(_ fleet.Todo, spec fleet.CycleSpec) (fleet.CycleSpec, bool) {
	if p.Freshen == nil {
		return spec, true
	}
	return p.Freshen(spec)
}

func (p Pool) transition(t fleet.PoolTransition) {
	fields := map[string]string{"live": strconv.Itoa(t.Live), "target": strconv.Itoa(t.Target)}
	reason := "pool " + string(t.Kind)
	if t.Todo != "" {
		fields["todo"] = t.Todo
		reason += " " + t.Todo
	}
	p.Publish.emit("Pool.Run", transitionCodes[t.Kind], signalcenter.SeverityInfo, reason, fields)
	if t.Kind == fleet.PoolIdle && p.MinLanes > 0 && t.Live < p.MinLanes && (p.Halted == nil || !p.Halted()) {
		uFields := map[string]string{
			"live":      strconv.Itoa(t.Live),
			"target":    strconv.Itoa(t.Target),
			"min_lanes": strconv.Itoa(p.MinLanes),
			"ready":     strconv.Itoa(t.Pending),
		}
		uReason := fmt.Sprintf("pool underfilled: %d of %d lanes live, min_lanes %d", t.Live, t.Target, p.MinLanes)
		p.Publish.emit("Pool.Run", CodePoolUnderfilled, signalcenter.SeverityWarn, uReason, uFields)
	}
}

func (p Pool) recover(lane Lane, ret Return) {
	if !ret.Stopped {
		return
	}
	if p.Reap != nil {
		p.Reap(lane)
	}
	if p.Claims == nil {
		return
	}
	todo := lane.Key
	p.check(todo, "release", p.Claims.Release(todo, releaseReason))
	if ret.Stops >= 2 {
		p.check(todo, "park", p.Claims.Park(todo, ParkReason))
	}
}

func (p Pool) check(todo, step string, err error) {
	if err == nil {
		return
	}
	p.Publish.emit("Pool.recover", CodeLaneRecoveryFailed, signalcenter.SeverityWarn,
		"orchestrator: "+step+" of "+todo+" after its deadline stop failed: "+err.Error(),
		map[string]string{"todo": todo, "step": step, "error": err.Error()})
}

func RecordShadow(publish Publish) func(admission.ShadowAdmission) {
	return func(s admission.ShadowAdmission) {
		publish.emit("admission.Shadow", CodeAdmissionShadow, signalcenter.SeverityInfo,
			fmt.Sprintf("optimistic admission would admit %s beside %s", s.ID, s.HeldBy),
			map[string]string{"todo": s.ID, "held_by": s.HeldBy, "rank": strconv.Itoa(s.Rank), "cells": strings.Join(s.Cells, ",")})
	}
}

func AdmitByCell(a admission.Admitter) Admit {
	return func(inFlight, pending []fleet.Todo) int {
		holdings := make([]admission.Holding, len(inFlight))
		for i, td := range inFlight {
			holdings[i] = admission.Holding{Lane: td.ID, Todo: td}
		}
		admitted, _ := a.Admit(holdings, pending, 1)
		if len(admitted) == 0 {
			return -1
		}
		return admitted[0].Rank
	}
}
