package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var errWriterRefused = errors.New("writer refused")

func TestRecover_EveryRecoveryRouteIsALegalEdgeFromShip(t *testing.T) {
	blockers := []router.Blocker{
		{Code: string(CodeGitFleetRebaseConflict), Class: "integrity"},
		{Code: "ANY", Class: "integrity"},
		{Code: "CONTROL_PLANE_VIOLATION"},
		{Code: "GIT_FF_MERGE_DIVERGED"},
		{Code: "GIT_PUSH_POLICY_REFUSED"},
		{Code: "EGPS_RED_COUNT"},
		{Code: string(CodeGitFleetRebaseNeeded)},
		{Code: "ANY", Class: "transient"},
		{Code: "ANY", Class: "unknown"},
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	reasons := map[string]bool{}
	for _, b := range blockers {
		dec := router.Recover(router.RouteInput{Blocker: &b})
		reasons[dec.Reason] = true
		cand := o.candidatePhase(dec.NextPhase)
		if cand != PhaseEnd && !o.sm.CanTransition(PhaseShip, cand) {
			t.Errorf("blocker %+v routes to %q (%s), which is not a legal edge from ship", b, cand, dec.Reason)
		}
	}
	if len(reasons) != 9 {
		t.Fatalf("the blockers reached %d recovery handlers %v, want all 9", len(reasons), reasons)
	}
}

func TestRecoverFromShipError_AFailedPendAfterAnUnwindAborts(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "pre-rebase"), []byte("#!/bin/sh\ngit update-ref -d refs/heads/main\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.git("config", "core.hooksPath", hooks)
	var next Phase
	var recovering bool

	stderr := captureStderr(t, func() { next, recovering, _ = fx.recover(&fakeStorage{}, fx.auditRow()) })

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want an abort: the change cannot be pended with no main", next, recovering)
	}
	if !strings.Contains(stderr, "unwound its ship commit") || !strings.Contains(stderr, "pend the audited change failed") {
		t.Fatalf("stderr does not show an unwind followed by a failed pend:\n%s", stderr)
	}
	if strings.Contains(stderr, "fleet rebase onto main failed (infra)") {
		t.Fatalf("the recovery went on past the failed pend:\n%s", stderr)
	}
}

func legacyCompositionRecovery(t *testing.T, writer func(CompositionVerdictInput) error) (Phase, bool, []string) {
	t.Helper()
	worktree, diff, patchID := divergedCompositionFixture(t)
	var methods []string
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(snapshotFromFixture(diff, patchID)),
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return allComposedGatesPass() }),
		WithCompositionVerdictWriter(func(_ string, in CompositionVerdictInput) error {
			methods = append(methods, in.Method)
			return writer(in)
		}),
	)
	cs := CycleState{CycleID: 7, RunID: "run-7", ActiveWorktree: worktree}
	next, recovering := o.recoverFromShipError(context.Background(), t.TempDir(), 7, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)
	return next, recovering, methods
}

func TestRecoverFromShipError_ALegacyCleanRebaseShipsOnTheTrivialRebaseRung(t *testing.T) {
	next, recovering, methods := legacyCompositionRecovery(t, func(CompositionVerdictInput) error { return nil })

	if !recovering || next != PhaseShip || len(methods) != 1 || methods[0] != "" {
		t.Fatalf("recovery=(%s,%v) records=%q, want Ship on one RUNG 0 record", next, recovering, methods)
	}
}

func TestRecoverFromShipError_ALegacyRebaseTheTrivialRungMissesShipsOnTheScopedReview(t *testing.T) {
	next, recovering, methods := legacyCompositionRecovery(t, func(in CompositionVerdictInput) error {
		if in.Method != scopedReviewMethod {
			return errWriterRefused
		}
		return nil
	})

	if !recovering || next != PhaseShip || len(methods) != 2 || methods[1] != scopedReviewMethod {
		t.Fatalf("recovery=(%s,%v) records=%q, want Ship on the RUNG 2 record after RUNG 0 declined", next, recovering, methods)
	}
}

func TestRecoverFromShipError_ARouteTheStateMachineForbidsAborts(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.sm = &StateMachine{allowed: map[Phase]map[Phase]bool{PhaseShip: {PhaseEnd: true}}}
	cs := CycleState{CycleID: 7, RunID: "run-7"}
	var next Phase
	var recovering bool

	stderr := captureStderr(t, func() {
		next, recovering = o.recoverFromShipError(context.Background(), t.TempDir(), 7, &cs,
			NewShipError("GIT_PUSH_TRANSIENT", ShipClassTransient, StageAtomicShip, "remote hiccup"), 0, 1)
	})

	if recovering || next != "" {
		t.Fatalf("recovery=(%s,%v), want an abort: ship→ship is not an edge of this state machine", next, recovering)
	}
	if !strings.Contains(stderr, "ship recovery proposed illegal edge ship→ship") || strings.Contains(stderr, "recovery routes to") {
		t.Fatalf("stderr does not show the illegal-edge abort alone:\n%s", stderr)
	}
}
