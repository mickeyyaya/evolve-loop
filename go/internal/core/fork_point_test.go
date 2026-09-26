package core

import (
	"context"
	"testing"
)

func TestRouteRebasedExplanation_BindsTheForkPointWhenMainMovesOn(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.git("rebase", "-q", "main")
	fx.landOnMain("later.txt", "a later landing\n")
	storage := &fakeStorage{}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
	cs := fx.cycleState()

	next, recovering := o.routeRebasedExplanation(context.Background(), fx.root, unwindCycle, &cs)

	if !recovering || next != PhaseBuild {
		t.Fatalf("route=(%s,%v), want Build: the rebased ship commit carries its inbox consumption", next, recovering)
	}
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("base memory=%s storage=%s, want the fork point %s, not the later main", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
}
