package core

import "context"

func (o *Orchestrator) RecoverFromShipErrorForTest(ctx context.Context, projectRoot string, cycle int, cs *CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	return o.recoverFromShipError(ctx, projectRoot, cycle, cs, se, depth, fleetWidth)
}

func (o *Orchestrator) RecoverWithCarryForTest(ctx context.Context, projectRoot string, cycle int, cs *CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	cr := &cycleRun{ctx: ctx, o: o, cycle: cycle, cs: *cs, req: CycleRequest{ProjectRoot: projectRoot}}
	carried, _ := cr.carryAuditedChangeOverMovedMain(se)
	next, recovering := o.recoverFromShipError(ctx, projectRoot, cycle, &cr.cs, carried, depth, fleetWidth)
	*cs = cr.cs
	return next, recovering
}
