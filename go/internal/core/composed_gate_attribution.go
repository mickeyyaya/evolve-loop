package core

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

type composedParkKey struct{}

type composedPark struct {
	Verdict baseattr.Verdict
	parked  bool
}

func withComposedPark(ctx context.Context) (context.Context, *composedPark) {
	p := &composedPark{}
	return context.WithValue(ctx, composedParkKey{}, p), p
}

func composedParkOf(ctx context.Context) *composedPark {
	p, _ := ctx.Value(composedParkKey{}).(*composedPark)
	return p
}

func (p *composedPark) Parked() bool { return p != nil && p.parked }

func (p *composedPark) shipError() *ShipError {
	return NewShipError(shiperr.CodeMainRedPark, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		"the red composed-tree gate tests are red on main too; the lane is not blamed and waits for a green main: "+baseattr.Names(p.Verdict.BaseRed),
		p.Verdict.Debug()...)
}

func (o *Orchestrator) composedBaseRunner() baseattr.Runner {
	if o.composedBaseRun != nil {
		return o.composedBaseRun
	}
	return baseattr.GoTestRunner(ipcenv.GoTestEnv())
}

func (o *Orchestrator) parkComposedRed(ctx context.Context, projectRoot string, cs CycleState, missing []string, outcomes map[string]ciparity.GateOutcome) bool {
	sink := composedParkOf(ctx)
	if sink == nil {
		return false
	}
	v, ok := o.attributeComposedReds(ctx, projectRoot, missing, outcomes)
	if !ok {
		return false
	}
	o.parkOnMainRed(projectRoot, cs, v)
	sink.Verdict, sink.parked = v, true
	return true
}

func (o *Orchestrator) attributeComposedReds(ctx context.Context, projectRoot string, missing []string, outcomes map[string]ciparity.GateOutcome) (baseattr.Verdict, bool) {
	attributor := BaseRedAttributor(projectRoot, o.composedBaseRunner(), o.errOut())
	var all baseattr.Verdict
	for _, gate := range missing {
		reds := outcomes[gate].Reds
		if !reds.Attributable() {
			return baseattr.Verdict{}, false
		}
		v, err := attributor.Attribute(ctx, baseattr.DefaultMainRef, reds)
		if err != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] composed-tree gate %s: the red could not be rerun on main (%v); the lane keeps the route of today\n", gate, err)
			return baseattr.Verdict{}, false
		}
		if v.Blamed() || len(v.MainRed) == 0 {
			return baseattr.Verdict{}, false
		}
		all.BaseRed = append(all.BaseRed, v.BaseRed...)
		all.MainRed = append(all.MainRed, v.MainRed...)
		all.Base, all.Main, all.Took = v.Base, v.Main, all.Took+v.Took
	}
	return all, true
}

func (cr *cycleRun) parkedOr(err error) error {
	if cr.composedPark.Parked() {
		return cr.composedPark.shipError()
	}
	return err
}
