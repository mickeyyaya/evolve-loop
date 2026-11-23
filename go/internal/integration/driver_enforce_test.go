package integration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type partialActor struct {
	fakeActor
	res FlushResult
	err error
}

func (a *partialActor) Flush(context.Context, PendingBatch) (FlushResult, error) {
	return a.res, a.err
}

type readyActor struct {
	fakeActor
	missing []string
}

func (a *readyActor) Missing() []string { return a.missing }

func TestDriverCheck_RefusesEnforceWithATypedReasonWhileAPortIsMissing(t *testing.T) {
	t.Parallel()
	for _, ports := range [][]string{{"Requeue"}, {"Promote"}, {"Briefs", "Gate"}} {
		r := newDriverRig(5)
		r.stage = policy.FleetIntegrationEnforce
		r.d.Actors[policy.FleetIntegrationEnforce] = &readyActor{missing: ports}

		err := r.d.Check()

		var refusal *MissingPortError
		if !errors.As(err, &refusal) || refusal.Stage != policy.FleetIntegrationEnforce || !slices.Equal(refusal.Ports, ports) {
			t.Fatalf("Check = %v, want a MissingPortError naming %v", err, ports)
		}
	}
}

func TestDriverActor_ReadinessIsReadThroughTheDriverOwnedInterface(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.stage = policy.FleetIntegrationEnforce
	r.d.Actors[policy.FleetIntegrationEnforce] = &readyActor{}
	if err := r.d.Check(); err != nil {
		t.Fatalf("Check of a ready actor = %v", err)
	}
	r.d.Actors[policy.FleetIntegrationEnforce] = &fakeActor{}
	if err := r.d.Check(); err != nil {
		t.Fatalf("Check of an actor with no readiness = %v", err)
	}
}

func TestDriverFlush_AMergeFollowedByAnErrorStillPersistsMerged(t *testing.T) {
	t.Parallel()
	batch := IntegrationBatch{Index: 1, Base: shaBase, Branch: PoolBranch(5, 1), Tip: shaTip1, PR: 7}
	failure := DriverFailure{Step: "promote", Error: "promote boom"}
	cases := map[string]struct {
		res     FlushResult
		wantA   MemberStatus
		wantB   MemberStatus
		batches int
	}{
		"a merge then a promote error": {
			res:   FlushResult{Batch: batch, Members: map[int]MemberResult{6: {Status: MemberMerged, Landed: true}, 7: {Status: MemberEjected}}, Failures: []DriverFailure{failure}},
			wantA: MemberMerged, wantB: MemberEjected, batches: 1,
		},
		"an error before any compose": {
			res:   FlushResult{},
			wantA: MemberPending, wantB: MemberPending, batches: 0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := newDriverRig(5, lane(5, 6, "a"), lane(5, 7, "b"))
			r.stage = policy.FleetIntegrationEnforce
			r.d.Actors[policy.FleetIntegrationEnforce] = &partialActor{res: tc.res, err: errors.New("later step failed")}

			err := r.d.EndWave(context.Background())

			if err == nil {
				t.Fatal("EndWave = nil, want the flush error")
			}
			rec := r.store.get(5)
			if rec.Members[6].Status != tc.wantA || rec.Members[7].Status != tc.wantB || len(rec.Batches) != tc.batches {
				t.Fatalf("members = %+v, batches = %d; want %s, %s and %d batch(es)", rec.Members, len(rec.Batches), tc.wantA, tc.wantB, tc.batches)
			}
			if tc.wantA == MemberMerged && (!rec.Members[6].Landed || len(rec.Failures) == 0 || len(rec.Decisions) == 0) {
				t.Fatalf("record = %+v, want the landed flag, the failure and the decision kept", rec)
			}
		})
	}
}

func TestDriverFlush_AFailedShadowFlushLeavesMembersPendingAndAddsOneDecision(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.actor.flushErr = errors.New("compose boom")

	if err := r.d.EndWave(context.Background()); err == nil {
		t.Fatal("EndWave = nil, want the flush error")
	}

	rec := r.store.get(5)
	if rec.Members[6].Status != MemberPending || len(rec.Batches) != 0 {
		t.Fatalf("members = %+v, batches = %d; want pending and no batch", rec.Members, len(rec.Batches))
	}
	var flushed int
	for _, d := range rec.Decisions {
		if slices.Contains(d.Flush, 6) {
			flushed++
		}
	}
	if flushed != 1 {
		t.Fatalf("decisions = %+v, want exactly one that flushed member 6", rec.Decisions)
	}
}

func TestDriver_AnEnforceFlushPublishesTheStageAndThePROfTheBatch(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.stage = policy.FleetIntegrationEnforce
	batch := IntegrationBatch{Index: 1, Base: shaBase, Branch: PoolBranch(5, 1), Tip: shaTip1, PR: 7, Members: []int{6}}
	r.d.Actors[policy.FleetIntegrationEnforce] = &partialActor{res: FlushResult{Batch: batch, Members: map[int]MemberResult{6: {Status: MemberMerged, Landed: true}}}}

	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	e := r.sink.events[0]
	if e.Fields["pr"] != "7" || e.Fields["shadow"] != "" || !strings.HasPrefix(e.Reason, "enforce: pool 5 batch 1 flushed") || !strings.HasSuffix(e.Reason, "with PR 7") {
		t.Fatalf("flushed event = %+v, want the enforce stage and PR 7, and no shadow field", e)
	}
}

func TestDriver_AFailureAtEnforceIsNotStampedShadow(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.stage = policy.FleetIntegrationEnforce
	r.d.Actors[policy.FleetIntegrationEnforce] = &readyActor{missing: []string{"Requeue"}}

	_ = r.d.Tick(context.Background())

	e := r.sink.events[0]
	if e.Fields["shadow"] != "" || !strings.HasPrefix(e.Reason, "enforce: ") {
		t.Fatalf("failure event = %+v, want the enforce stage and no shadow field", e)
	}
}

func TestDriver_AStageThatCannotBeReadIsLabelledShadow(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	if got := d.stageLabel(); got != policy.FleetIntegrationShadow {
		t.Fatalf("stageLabel with no Stage = %q", got)
	}
	d.Stage = func() (string, error) { return "", errors.New("bad policy") }
	if got := d.stageLabel(); got != policy.FleetIntegrationShadow {
		t.Fatalf("stageLabel on an error = %q", got)
	}
}
