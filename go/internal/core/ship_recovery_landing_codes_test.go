package core_test

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRecoverFromShipError_TheLandingStopCodesEndTheCycleWithoutAReaudit(t *testing.T) {
	for code, class := range map[core.ShipErrorCode]core.ShipErrorClass{
		core.CodeGitLaneNotOnOrigin:       core.ShipClassIntegrity,
		core.CodeGitLandingUnwindDeclined: core.ShipClassIntegrity,
		core.CodeGitPushPolicyRefused:     core.ShipClassPrecondition,
	} {
		o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil))
		cs := core.CycleState{CycleID: 1830, RunID: "run-1830", WorkspacePath: t.TempDir()}
		se := core.NewShipError(code, class, core.StageAtomicShip, "the landing cannot complete")

		next, recovered := o.RecoverFromShipErrorForTest(context.Background(), t.TempDir(), 1830, &cs, se, 0, 1)

		if recovered || next != "" {
			t.Errorf("%s routed to %q (recovered=%v): a re-audit cannot change this landing's outcome, so the cycle stops with its work kept", code, next, recovered)
		}
	}
}
