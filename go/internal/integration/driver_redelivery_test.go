package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func redeliverAfterCompose(t *testing.T, stage string) WaveRecord {
	t.Helper()
	r := newDriverRig(5, lane(5, 6, "first"))
	r.stage = stage
	r.d.Actors[stage] = r.actor
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r.store.get(5)
}

func TestDriver_AtShadowARedeliveryIsInformationOnTheComposedMember(t *testing.T) {
	t.Parallel()
	rec := redeliverAfterCompose(t, policy.FleetIntegrationShadow)
	m := rec.Members[6]
	if m.Status != MemberComposed || m.PatchIDs[0] != "first" {
		t.Fatalf("member = %+v, want the composed member kept", m)
	}
	if len(m.Redelivered) != 1 || m.Redelivered[0] != "second" {
		t.Fatalf("redelivered = %v, want the second patch ids", m.Redelivered)
	}
	if len(rec.Failures) != 0 {
		t.Fatalf("failures = %+v, want none", rec.Failures)
	}
}

func TestDriver_AtEnforceARedeliveryAfterAComposeIsRecordedAsAFailure(t *testing.T) {
	t.Parallel()
	rec := redeliverAfterCompose(t, policy.FleetIntegrationEnforce)
	if m := rec.Members[6]; m.Status != MemberComposed || m.PatchIDs[0] != "first" || m.Redelivered != nil {
		t.Fatalf("member = %+v, want the composed member kept and no information", m)
	}
	if len(rec.Failures) != 1 || !strings.Contains(rec.Failures[0].Error, "cycle 6") {
		t.Fatalf("failures = %+v, want the redelivery named once", rec.Failures)
	}
}

func TestDriver_AStageThatCannotBeReadKeepsTheFailure(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.d.Stage = func() (string, error) { return "", errors.New("unreadable") }
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	r.d.admit()
	if len(r.d.rec.Failures) != 1 {
		t.Fatalf("failures = %+v, want the failure kept", r.d.rec.Failures)
	}
}

func TestDriver_APendingRedeliveryIsNoFailure(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f := r.store.get(5).Failures; len(f) != 0 {
		t.Fatalf("failures = %+v, want none", f)
	}
}

func TestDriver_APendingRedeliveryAtEnforceIsNoFailure(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	r.stage = policy.FleetIntegrationEnforce
	r.d.Actors[policy.FleetIntegrationEnforce] = r.actor
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f := r.store.get(5).Failures; len(f) != 0 {
		t.Fatalf("failures = %+v, want none", f)
	}
}

func TestDriver_AComposedMemberSeenAgainUnderItsOwnPatchesIsNoFailure(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	for range 2 {
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if f := r.store.get(5).Failures; len(f) != 0 {
		t.Fatalf("failures = %+v, want none", f)
	}
	if got := r.store.get(5).Members[6].Redelivered; got != nil {
		t.Fatalf("redelivered = %v, want none", got)
	}
}

func TestDriver_AnOlderPoolsRecordOfAComposedCycleIsNoFailure(t *testing.T) {
	t.Parallel()
	r := newDriverRig(8, lane(5, 6, "first"), lane(8, 6, "second"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberComposed, PatchIDs: []string{"first"}}}}
	for range 2 {
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	rec := r.store.get(8)
	if m := rec.Members[6]; m.Status != MemberComposed || m.PatchIDs[0] != "second" {
		t.Fatalf("member = %+v, want the second patch composed", m)
	}
	if len(rec.Failures) != 0 {
		t.Fatalf("failures = %+v, want none", rec.Failures)
	}
}

func TestPromotionReachableByGoodBehavior(t *testing.T) {
	t.Parallel()
	for stage, wantFailures := range map[string]int{policy.FleetIntegrationShadow: 0, policy.FleetIntegrationEnforce: 4} {
		r := newDriverRig(2378, lane(2378, 2379, "a"), lane(2378, 2382, "b"), lane(2378, 2383, "c"), lane(2378, 2386, "d"))
		r.stage = stage
		r.d.Actors[stage] = r.actor
		r.d.Flush = FlushPolicy{Cap: 4, MaxAgeMinutes: 600}
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		r.set.Listed = []ListedRecord{lane(2378, 2379, "a2"), lane(2378, 2382, "b2"), lane(2378, 2383, "c2"), lane(2378, 2386, "d2")}
		if err := r.d.EndWave(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := len(r.store.get(2378).Failures); got != wantFailures {
			t.Fatalf("stage %s: failures = %d, want %d", stage, got, wantFailures)
		}
	}
}
