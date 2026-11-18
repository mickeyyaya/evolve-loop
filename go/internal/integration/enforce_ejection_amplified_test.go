package integration

import (
	"context"
	"slices"
	"testing"
	"time"
)

func ejectingActor(dir string, publish *sink) EnforceActor {
	a := EnforceActor{
		Merger: Merger{
			CI:   stubCI{verdict: Red},
			Main: &stubMain{head: testSHA},
			Compose: func(_ context.Context, recs []DeliverableRecord) (Composition, error) {
				return Composition{Batch: Batch{Base: testSHA, Tip: testSHA}, Kept: recs}, nil
			},
		},
		Briefs:  briefDir{dir: dir},
		Requeue: &stubEnforcePorts{},
		Now:     func() time.Time { return time.Unix(100, 0).UTC() },
	}
	if publish != nil {
		a.Publish = publish
	}
	return a
}

func TestEnforceActorAmplified_ANilPublishStillEjectsAndWritesTheBrief(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := ejectingActor(dir, nil)

	res, err := a.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: testSHA, Members: []DeliverableRecord{{Cycle: 9}}})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if res.Members[9].Status != MemberEjected {
		t.Fatalf("member 9 = %+v, want ejected", res.Members[9])
	}
	if _, err := ReadBrief(BriefPath(dir, 9)); err != nil {
		t.Errorf("brief unreadable with a nil Publish: %v", err)
	}
}

func TestEnforceActorAmplified_EachEjectedMemberGetsExactlyOneEventOfItsOwnCycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := &sink{}
	a := ejectingActor(dir, out)

	res, err := a.Flush(context.Background(), PendingBatch{Pool: 1, Index: 1, Base: testSHA, Members: []DeliverableRecord{{Cycle: 9}, {Cycle: 10}}})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	var ejected []int
	for c, m := range res.Members {
		if m.Status == MemberEjected {
			ejected = append(ejected, c)
		}
	}
	if len(ejected) == 0 {
		t.Fatal("no member ejected, want the red CI to eject")
	}
	out.mu.Lock()
	defer out.mu.Unlock()
	var evented []int
	for _, e := range out.events {
		if e.Code == CodeEjected {
			evented = append(evented, e.Cycle)
		}
	}
	slices.Sort(ejected)
	slices.Sort(evented)
	if !slices.Equal(evented, ejected) {
		t.Errorf("ejected events by cycle = %v, want exactly one per ejected cycle %v", evented, ejected)
	}
}
