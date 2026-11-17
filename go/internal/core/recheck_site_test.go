package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

func TestRecheckSite_RoundTripsThroughTheContext(t *testing.T) {
	t.Parallel()
	if _, ok := RecheckSiteFrom(context.Background()); ok {
		t.Fatal("a bare context names a re-check site")
	}
	want := RecheckSite{ProjectRoot: "/plane", Cycle: 9, AuditedBase: "abc123"}
	got, ok := RecheckSiteFrom(WithRecheckSite(context.Background(), want))
	if !ok || got != want {
		t.Fatalf("RecheckSiteFrom = (%+v, %v), want (%+v, true)", got, ok, want)
	}
}

func TestRecheckSite_IdentityCarryHandsTheGateRunnerTheAuditedBaseAndTheProjectRoot(t *testing.T) {
	t.Parallel()
	h := pendedIdenticalLane(t)

	next, recovering, _ := h.route(t, h.auditRow())

	if !recovering || next != PhaseShip {
		t.Fatalf("route=(%s,%v), want the identity carry to reach Ship", next, recovering)
	}
	want := RecheckSite{ProjectRoot: h.fx.root, Cycle: unwindCycle, AuditedBase: h.fx.base}
	if !h.siteStamped || h.site != want {
		t.Errorf("the composed-gate runner saw site (%+v, stamped=%v), want %+v", h.site, h.siteStamped, want)
	}
}

func TestRecheckSite_ScopedMergeHandsTheGateRunnerTheAuditedBaseAndTheProjectRoot(t *testing.T) {
	t.Parallel()
	worktree, diff, patchID := divergedCompositionFixture(t)
	snapshot := func(ctx context.Context, wt, runID string) (CompositionAuditSnapshot, error) {
		snap, err := snapshotFromFixture(diff, patchID)(ctx, wt, runID)
		snap.AuditedBase = "audited-base-sha"
		return snap, err
	}
	var seen RecheckSite
	var stamped bool
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil),
		WithCompositionSnapshot(snapshot),
		WithCompositionGateRunner(func(ctx context.Context, _ string) map[string]ciparity.GateOutcome {
			seen, stamped = RecheckSiteFrom(ctx)
			return allComposedGatesPass()
		}),
		WithCompositionVerdictWriter(func(string, CompositionVerdictInput) error { return nil }),
		WithScopedMergeReviewer(func([]MergeHunk, string, string) ScopedMergeReviewOutcome {
			return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
		}),
	)

	if !o.scopedMergeCarryForward(context.Background(), 7, CycleState{ActiveWorktree: worktree}, "/plane") {
		t.Fatal("the scoped merge did not carry; the gate runner was never reached")
	}

	want := RecheckSite{ProjectRoot: "/plane", Cycle: 7, AuditedBase: "audited-base-sha"}
	if !stamped || seen != want {
		t.Errorf("the composed-gate runner saw site (%+v, stamped=%v), want %+v", seen, stamped, want)
	}
}
