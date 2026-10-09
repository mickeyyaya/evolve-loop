package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	consumedItem     = ".evolve/inbox/consumed/item.json"
	releasedItemBody = "{\"id\":\"item\",\"consumed\":{\"at\":\"2026-10-09T03:24:00Z\",\"via\":\"ship\",\"cycle\":\"42\"},\"released_continuations\":[{\"cycle\":41,\"snapshot_sha\":\"61ec214b6\"}]}\n"
)

func newContinuationLane(t *testing.T) *shippedLane {
	t.Helper()
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.write(consumedItem, releasedItemBody)
	fx.git("add", "-A")
	fx.git("commit", "-q", "--amend", "-m", "ship cycle 42")
	fx.shipCommit = fx.git("rev-parse", "HEAD")
	return fx
}

func recordSignals(center *signalcenter.Center) *[]signalcenter.Event {
	events := &[]signalcenter.Event{}
	center.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })
	return events
}

func codedEvents(events []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

func TestRouteRebasedExplanation_ACommittedChangeReportsTheProofSkipped(t *testing.T) {
	fx := rebasedCommittedLane(t)
	center := signalcenter.New()
	events := recordSignals(center)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))
	cs := fx.cycleState()
	var next Phase
	var recovering bool

	stderr := captureStderr(t, func() {
		next, recovering = o.routeRebasedExplanation(context.Background(), fx.root, unwindCycle, &cs)
	})

	if !recovering || next != PhaseBuild {
		t.Fatalf("route=(%s,%v), want Build: a committed change cannot be rebound", next, recovering)
	}
	if strings.Contains(stderr, "not proven identical") {
		t.Errorf("stderr says the proof failed, but the proof did not run:\n%s", stderr)
	}
	if !strings.Contains(stderr, "identity proof skipped") || !strings.Contains(stderr, string(CodeIdentityProofSkipped)) {
		t.Errorf("stderr does not report the skipped proof with its code:\n%s", stderr)
	}
	skipped := codedEvents(*events, CodeIdentityProofSkipped)
	if len(skipped) != 1 {
		t.Fatalf("events = %+v, want one %s event", *events, CodeIdentityProofSkipped)
	}
	e := skipped[0]
	if e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleOrchestrator || e.Cycle != unwindCycle || e.RunID != unwindRunID {
		t.Errorf("event = %+v, want a WARN of module orchestrator for cycle %d run %s", e, unwindCycle, unwindRunID)
	}
	if reason := e.Fields["reason"]; !strings.Contains(reason, "committed") || !strings.Contains(reason, fx.newBase) {
		t.Errorf("fields.reason = %q, want it to say the change is committed, not pending on %s", reason, fx.newBase)
	}
}

func TestRecoverFromShipError_AContinuationLaneRunsTheIdentityProof(t *testing.T) {
	fx := newContinuationLane(t)
	storage := &fakeStorage{}
	var next Phase
	var recovering bool
	var cs CycleState

	stderr := captureStderr(t, func() { next, recovering, cs = fx.recover(storage, fx.auditRow()) })

	if !recovering || next != PhaseAudit {
		t.Fatalf("recovery=(%s,%v), want Audit with no Build: the rebase of a disjoint lane is byte-identical\n%s", next, recovering, stderr)
	}
	if !strings.Contains(stderr, "ship unwind declined") || strings.Contains(stderr, "not proven identical") || strings.Contains(stderr, "identity proof skipped") {
		t.Errorf("stderr does not show a declined unwind followed by a proof that ran:\n%s", stderr)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.newBase {
		t.Fatalf("HEAD = %s, want the change pending on the new base %s", head, fx.newBase)
	}
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("rebound base not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
	view, ok, err := explanationdocs.Verify(context.Background(), fx.bindingAt(fx.newBase))
	if err != nil || !ok {
		t.Fatalf("the rebound explanation does not verify on the new base: ok=%v err=%v", ok, err)
	}
	if view.AuthoredBaseSHA != fx.base {
		t.Fatalf("authored base = %q, want %q", view.AuthoredBaseSHA, fx.base)
	}
}

func TestRecoverFromShipError_AContinuationLaneKeepsItsReleasedContinuation(t *testing.T) {
	fx := newContinuationLane(t)

	fx.recover(&fakeStorage{}, fx.auditRow())

	pending := fx.git("diff", "--cached", "--name-status", "--no-renames", "HEAD")
	for _, want := range []string{"D\t" + inboxItem, "A\t" + consumedItem} {
		if !strings.Contains(pending, want) {
			t.Fatalf("pending change =\n%s\nwant ship's consumption %q in the change Audit binds and the next ship commits", pending, want)
		}
	}
	if body := fx.git("show", ":"+consumedItem); body+"\n" != releasedItemBody {
		t.Fatalf("staged consumed item = %q, want ship's bytes with the released continuation %q", body, releasedItemBody)
	}
}

func TestRecoverFromShipError_AContinuationLaneWritesNoCarryShipCannotReProve(t *testing.T) {
	fx := newContinuationLane(t)
	h := &carryHarness{fx: fx, gates: allComposedGatesPass()}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{h.auditRow()}}, buildRunners(nil),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref"}, nil
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return h.gates }),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error {
			t.Error("a carry record was written for a change that holds ship's consumption; ship re-proves the bytes and refuses it")
			return nil
		}),
	)
	cs := fx.cycleState()

	next, recovering := o.recoverFromShipError(context.Background(), fx.root, unwindCycle, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)

	if !recovering || next != PhaseAudit {
		t.Fatalf("recovery=(%s,%v), want Audit: the explanation is rebound, and Audit binds the tree with the consumption", next, recovering)
	}
}

func TestRecoverFromShipError_AContinuationLaneWhoseRebaseFailsKeepsShipsCommit(t *testing.T) {
	fx := newContinuationLane(t)
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "pre-rebase"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.git("config", "core.hooksPath", hooks)

	next, recovering, _ := fx.recover(&fakeStorage{}, fx.auditRow())

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want an abort: the rebase failed with no conflict", next, recovering)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Fatalf("HEAD = %s, want ship's commit %s left as it was: only a clean replay is pended", head, fx.shipCommit)
	}
}

func TestRebindPendingChange_AnUnreadableHeadIsAnError(t *testing.T) {
	binding := explanationdocs.CycleBinding{Worktree: t.TempDir()}

	rebound, skipped, err := rebindPendingChange(context.Background(), binding, strings.Repeat("a", 40), func() error { return nil })

	if err == nil || rebound || skipped != "" {
		t.Fatalf("rebindPendingChange = (%v, %q, %v), want an error and no skip report for a worktree git cannot read", rebound, skipped, err)
	}
}

func TestRouteRebasedExplanation_AnUnresolvedBaseAborts(t *testing.T) {
	fx := rebasedCommittedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := fx.cycleState()
	cs.ActiveWorktree = t.TempDir()

	next, recovering := o.routeRebasedExplanation(context.Background(), fx.root, unwindCycle, &cs)

	if recovering || next != "" {
		t.Fatalf("route=(%s,%v), want an abort: git cannot resolve the fork point", next, recovering)
	}
}

func TestRouteRebasedExplanation_AContractTheHostCannotReadAborts(t *testing.T) {
	fx := rebasedCommittedLane(t)
	fx.git("reset", "-q", "--soft", fx.newBase)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cs := fx.cycleState()

	next, recovering := o.routeRebasedExplanation(context.Background(), t.TempDir(), unwindCycle, &cs)

	if recovering || next != "" {
		t.Fatalf("route=(%s,%v), want an abort: no activated contract exists under the project root", next, recovering)
	}
}

func TestRecoverFromShipError_ALaneAlreadyOnMainEndsWithNoReplay(t *testing.T) {
	fx := newContinuationLane(t)
	fx.git("branch", "-f", "main", "cycle")

	next, recovering, _ := fx.recover(&fakeStorage{}, fx.auditRow())

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want an end: main already holds the lane", next, recovering)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Fatalf("HEAD = %s, want ship's commit %s untouched", head, fx.shipCommit)
	}
}

func TestRecoverFromShipError_APreScreenFaultFallsThroughToTheRebase(t *testing.T) {
	fx := newContinuationLane(t)
	fx.git("branch", "-m", "main", "trunk")

	next, recovering, _ := fx.recover(&fakeStorage{}, fx.auditRow())

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want an abort: with no main the rebase fails as infra", next, recovering)
	}
}
