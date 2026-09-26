package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

// rebasedCommittedLane is a lane whose ship commit carried no consumption, replayed onto the new base.
func rebasedCommittedLane(t *testing.T) *shippedLane {
	t.Helper()
	fx := newCommittedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.git("rebase", "-q", "main")
	return fx
}

func routeExplanation(fx *shippedLane, storage *fakeStorage) (Phase, bool, CycleState) {
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
	cs := fx.cycleState()
	next, recovering := o.routeRebasedExplanation(context.Background(), fx.root, unwindCycle, &cs)
	return next, recovering, cs
}

func TestRouteRebasedExplanation_APendingIdenticalChangeReturnsToAudit(t *testing.T) {
	fx := rebasedCommittedLane(t)
	fx.git("reset", "-q", "--soft", fx.newBase)
	storage := &fakeStorage{}

	next, recovering, cs := routeExplanation(fx, storage)

	if !recovering || next != PhaseAudit {
		t.Fatalf("route=(%s,%v), want Audit", next, recovering)
	}
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("rebound base not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
	if _, ok, err := explanationdocs.Verify(context.Background(), fx.bindingAt(fx.newBase)); err != nil || !ok {
		t.Fatalf("the rebound explanation does not verify on the new base: ok=%v err=%v", ok, err)
	}
}

// Audit reads `git diff HEAD`: a committed change would show it an empty diff, so only Build, whose
// normalisation pends the change, may follow.
func TestRouteRebasedExplanation_ACommittedIdenticalChangeReturnsToBuild(t *testing.T) {
	fx := rebasedCommittedLane(t)

	next, recovering, cs := routeExplanation(fx, &fakeStorage{})

	if !recovering || next != PhaseBuild {
		t.Fatalf("route=(%s,%v), want Build: the change is committed, so Audit would see no diff", next, recovering)
	}
	if cs.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("base = %s, want the fork point %s", cs.WorktreeBaseSHA, fx.newBase)
	}
}

func TestRouteRebasedExplanation_AnIncompleteRebindAborts(t *testing.T) {
	fx := rebasedCommittedLane(t)
	fx.git("reset", "-q", "--soft", fx.newBase)

	next, recovering, cs := routeExplanation(fx, &fakeStorage{writeCSFailAt: 1})

	if recovering || next != "" {
		t.Fatalf("route=(%s,%v), want an abort: the rebind wrote the marker and then failed", next, recovering)
	}
	if cs.WorktreeBaseSHA != fx.base {
		t.Fatalf("in-memory base moved to %q on a failed rebind", cs.WorktreeBaseSHA)
	}
}
