package integration

import (
	"context"
	"strings"
	"testing"
)

func TestDriver_ARedeliveryAfterAComposeIsRecordedAndTheComposedMemberStays(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	r.d.Flush = FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := r.store.get(5)
	if m := rec.Members[6]; m.Status != MemberComposed || m.PatchIDs[0] != "first" {
		t.Fatalf("member = %+v, want the composed member kept", m)
	}
	if len(rec.Failures) != 1 || !strings.Contains(rec.Failures[0].Error, "cycle 6") {
		t.Fatalf("failures = %+v, want the redelivery named once", rec.Failures)
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
