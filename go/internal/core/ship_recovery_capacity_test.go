package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func capacityError() *ShipError {
	return NewShipError(shiperr.CodeRepoContractCapacity, ShipClassTransient, StageAtomicShip, "importer backstop hit its deadline")
}

func recoverCapacity(t *testing.T, cs CycleState, se *ShipError) (Phase, bool, []signalcenter.Event, string) {
	t.Helper()
	center, events := captureSignals()
	var stderr injectedStderr
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center), WithStderr(&stderr))
	cs.CycleID, cs.RunID, cs.WorkspacePath = 7, "run-7", t.TempDir()
	next, recovering := o.recoverFromShipError(context.Background(), t.TempDir(), 7, &cs, se, 0, 2)
	return next, recovering, *events, stderr.String()
}

func parkSignals(events []signalcenter.Event) []signalcenter.Event {
	var parks []signalcenter.Event
	for _, e := range events {
		if e.Code == CodeCapacityParked {
			parks = append(parks, e)
		}
	}
	return parks
}

func TestRecovery_ADeadlineKillOnAnUnchangedTreeRetriesWithoutReaudit(t *testing.T) {
	t.Parallel()

	next, recovering, events, stderr := recoverCapacity(t, CycleState{}, capacityError())

	if !recovering || next != PhaseShip {
		t.Fatalf("recovery=(%s,%v), want the gate retried through Ship with no re-audit", next, recovering)
	}
	if len(parkSignals(events)) != 0 {
		t.Fatalf("the first kill parked the lane: %+v", events)
	}
	if strings.Contains(stderr, "routes to audit") {
		t.Fatalf("the kill went to an LLM re-audit:\n%s", stderr)
	}
}

func TestRecovery_ASecondKillParksTheLaneWithACapacityCode(t *testing.T) {
	t.Parallel()

	cs := CycleState{ShipRecoveryCode: string(shiperr.CodeRepoContractCapacity)}
	next, recovering, events, stderr := recoverCapacity(t, cs, capacityError())

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want the lane parked after a second kill", next, recovering)
	}
	parks := parkSignals(events)
	if len(parks) != 1 {
		t.Fatalf("signals = %+v, want one %s", events, CodeCapacityParked)
	}
	p := parks[0]
	if p.Fields["code"] != string(shiperr.CodeRepoContractCapacity) || p.Cycle != 7 || p.Severity != signalcenter.SeverityWarn || !strings.Contains(p.Reason, "next:") {
		t.Errorf("park signal = %+v, want the capacity code, the cycle, a warn severity and a next step", p)
	}
	if !strings.Contains(stderr, "parked") {
		t.Errorf("stderr does not state the park:\n%s", stderr)
	}
}

func TestRecovery_AnotherFailureAfterACapacityKillKeepsItsRoute(t *testing.T) {
	t.Parallel()

	cs := CycleState{ShipRecoveryCode: string(shiperr.CodeRepoContractCapacity)}
	infra := NewShipError(shiperr.CodeRepoContractInfra, ShipClassPrecondition, StageAtomicShip, "pack died twice")
	next, recovering, events, _ := recoverCapacity(t, cs, infra)

	if !recovering || next != PhaseAudit {
		t.Fatalf("recovery=(%s,%v), want the infra route of today (re-audit)", next, recovering)
	}
	if len(parkSignals(events)) != 0 {
		t.Fatalf("a non-capacity failure parked the lane: %+v", events)
	}
}

func TestRecovery_ACapacityKillAfterAnotherRecoveryRetriesAgain(t *testing.T) {
	t.Parallel()

	cs := CycleState{ShipRecoveryCode: string(CodeGitFleetRebaseNeeded)}
	next, recovering, events, _ := recoverCapacity(t, cs, capacityError())

	if !recovering || next != PhaseShip || len(parkSignals(events)) != 0 {
		t.Fatalf("recovery=(%s,%v) signals=%+v, want a retry: the tree changed since the last kill", next, recovering, events)
	}
}
