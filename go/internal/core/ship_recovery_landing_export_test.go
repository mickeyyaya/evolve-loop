package core

import "context"

func (o *Orchestrator) RecoverFromShipErrorForTest(ctx context.Context, projectRoot string, cycle int, cs *CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	return o.recoverFromShipError(ctx, projectRoot, cycle, cs, se, depth, fleetWidth)
}
