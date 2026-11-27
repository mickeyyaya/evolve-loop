package integration

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func onceRecord(cycle int, patch, sha string) DeliverableRecord {
	return DeliverableRecord{
		Wave: "wave-1", Cycle: cycle, Trailer: CycleTrailer(cycle),
		Commits:     []CommitIdentity{{SHA: sha, BaseSHA: "b0", PatchID: patch}},
		Audit:       AuditBinding{TreeSHA: "t1", ArtifactSHA: "art1"},
		Scope:       []string{"go/internal/a"},
		Risk:        "low",
		DeliveredAt: time.Unix(300, 0).UTC(),
	}
}

func deliveredOnce(t *testing.T, rec DeliverableRecord) (dir, path string, before []byte, stamp time.Time) {
	t.Helper()
	dir = t.TempDir()
	path, err := WriteRecordWith(dir, rec, WriteOnce)
	if err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	stamp = time.Unix(1_000, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if before, err = os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	return dir, path, before, stamp
}

func requireRecordUntouched(t *testing.T, dir, path string, before []byte, stamp time.Time) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("record changed after the second delivery: err=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("record mtime = %v, want the first write %v (err=%v)", info.ModTime(), stamp, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("wave directory holds %d entries (err=%v), want only the record and no *.tmp", len(entries), err)
	}
}

func raceWithFirstWriter(t *testing.T, dir string, first DeliverableRecord) {
	t.Helper()
	prev := createRecordTemp
	t.Cleanup(func() { createRecordTemp = prev })
	armed := true
	createRecordTemp = func(d, pattern string) (*os.File, error) {
		if armed {
			armed = false
			if _, err := WriteRecordWith(dir, first, WriteOnce); err != nil {
				t.Errorf("the first writer of the race: %v", err)
			}
		}
		return prev(d, pattern)
	}
}

func TestWriteOnce_RowRa_ASecondDeliveryWithTheSamePatchIDsIsANoOp(t *testing.T) {
	first := onceRecord(12, "p1", "sha-first")
	dir, path, before, stamp := deliveredOnce(t, first)

	got, err := WriteRecordWith(dir, onceRecord(12, "p1", "sha-after-retry"), WriteOnce)

	if err != nil || got != path {
		t.Fatalf("same-patch retry = (%q, %v), want a no-op that returns %q", got, err, path)
	}
	requireRecordUntouched(t, dir, path, before, stamp)
}

func TestWriteOnce_RowRb_ASecondDeliveryWithNewPatchIDsIsRefusedWithATypedErrorNamingTheMember(t *testing.T) {
	dir, path, before, stamp := deliveredOnce(t, onceRecord(12, "p1", "sha-first"))

	_, err := WriteRecordWith(dir, onceRecord(12, "p2", "sha-second"), WriteOnce)

	var refused *RecordRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("new patches = %v, want a *RecordRefusedError", err)
	}
	if refused.Member != "cycle 12" || refused.Path != path {
		t.Errorf("refusal = %+v, want member %q and path %q", refused, "cycle 12", path)
	}
	if !errors.Is(err, ErrRecordExists) {
		t.Errorf("a refusal must still unwrap to ErrRecordExists: %v", err)
	}
	requireRecordUntouched(t, dir, path, before, stamp)
}

func TestWriteOnce_PatchIDsAreComparedAsAWholeSetNotByPrefix(t *testing.T) {
	two := onceRecord(12, "p1", "s1")
	two.Commits = append(two.Commits, CommitIdentity{SHA: "s2", BaseSHA: "b0", PatchID: "p2"})
	dir, path, before, stamp := deliveredOnce(t, two)

	_, err := WriteRecordWith(dir, onceRecord(12, "p1", "s1"), WriteOnce)

	var refused *RecordRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("a record with fewer commits = %v, want a refusal", err)
	}
	requireRecordUntouched(t, dir, path, before, stamp)
}

func TestWriteOnce_ASecondDeliveryReadsTheRecordFileNotTheWaveMember(t *testing.T) {
	dir, path, before, stamp := deliveredOnce(t, onceRecord(12, "p1", "sha-first"))
	if _, err := WriteRecordWith(dir, onceRecord(13, "p9", "other"), WriteOnce); err != nil {
		t.Fatalf("another member must deliver freely: %v", err)
	}
	if _, err := WriteRecordWith(dir, onceRecord(12, "p1", "again"), WriteOnce); err != nil {
		t.Fatalf("same patches of member 12 after member 13 wrote: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("record 12 rewritten (err=%v)", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("record 12 changed")
	}
}

func TestWriteOnce_GuardTable_EveryShipPathAtEnforce(t *testing.T) {
	paths := []struct {
		name   string
		shaNew bool
	}{
		{"ship retry HEAD_MOVED", true},
		{"FLEET_REBASE_NEEDED", true},
		{"moveLaneRef failure, then retry", true},
		{"evolve loop --resume", false},
	}
	for _, p := range paths {
		t.Run(p.name+"/same patches is a no-op", func(t *testing.T) {
			sha := "sha-first"
			if p.shaNew {
				sha = "sha-rebuilt"
			}
			dir, path, before, stamp := deliveredOnce(t, onceRecord(12, "p1", "sha-first"))
			got, err := WriteRecordWith(dir, onceRecord(12, "p1", sha), WriteOnce)
			if err != nil || got != path {
				t.Fatalf("= (%q, %v), want a no-op", got, err)
			}
			requireRecordUntouched(t, dir, path, before, stamp)
		})
		t.Run(p.name+"/new patches is a typed refusal", func(t *testing.T) {
			dir, path, before, stamp := deliveredOnce(t, onceRecord(12, "p1", "sha-first"))
			_, err := WriteRecordWith(dir, onceRecord(12, "p-new", "sha-rebuilt"), WriteOnce)
			var refused *RecordRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("= %v, want a typed refusal", err)
			}
			requireRecordUntouched(t, dir, path, before, stamp)
		})
	}
}

func TestWriteOnce_GuardTable_ARaceUnderExclusiveCreateKeepsTheFirstRecord(t *testing.T) {
	cases := []struct {
		name    string
		second  DeliverableRecord
		refused bool
	}{
		{"concurrent second writer with the same patches", onceRecord(12, "p1", "sha-second"), false},
		{"concurrent second writer with new patches", onceRecord(12, "p2", "sha-second"), true},
		{"driver-admit race: a second delivery before the driver admits the member", onceRecord(12, "p3", "sha-second"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			first := onceRecord(12, "p1", "sha-first")
			raceWithFirstWriter(t, dir, first)

			_, err := WriteRecordWith(dir, c.second, WriteOnce)

			var refused *RecordRefusedError
			if got := errors.As(err, &refused); got != c.refused || (!c.refused && err != nil) {
				t.Fatalf("race outcome = %v, want refusal=%v", err, c.refused)
			}
			held, rerr := ReadRecord(RecordPath(dir, 12))
			if rerr != nil || held.Commits[0].SHA != "sha-first" {
				t.Fatalf("the first record did not stay: %+v (%v)", held.Commits, rerr)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 1 {
				t.Fatalf("directory holds %d entries, want 1", len(entries))
			}
		})
	}
}

func TestWriteOnce_TheReplacePolicyStillReplacesAfterTheDefence(t *testing.T) {
	dir, path, _, _ := deliveredOnce(t, onceRecord(12, "p1", "sha-first"))
	if _, err := WriteRecordWith(dir, onceRecord(12, "p2", "sha-second"), ReplaceExisting); err != nil {
		t.Fatalf("shadow replace: %v", err)
	}
	got, err := ReadRecord(path)
	if err != nil || got.Commits[0].PatchID != "p2" {
		t.Fatalf("replace = %+v, %v, want the new patches on disk", got.Commits, err)
	}
}

func enforceRig(status MemberStatus, flushCap int, first ListedRecord) *driverRig {
	r := newDriverRig(5, first)
	r.stage = policy.FleetIntegrationEnforce
	r.actor.status = status
	r.d.Actors[policy.FleetIntegrationEnforce] = r.actor
	r.d.Flush = FlushPolicy{Cap: flushCap, MaxAgeMinutes: 600}
	return r
}

func flushedPatchIDs(a *fakeActor) []string {
	var out []string
	for _, b := range a.flushes {
		for _, m := range b.Members {
			out = append(out, patchIDsOf(m)...)
		}
	}
	return out
}

func TestDriverAdmits_RowRb_AtEnforceANewPatchSetNeverReplacesAMemberInAnyStatus(t *testing.T) {
	for _, status := range []MemberStatus{MemberPending, MemberComposed, MemberMerged, MemberEjected} {
		t.Run(string(status), func(t *testing.T) {
			flushCap := 1
			if status == MemberPending {
				flushCap = 10
			}
			r := enforceRig(status, flushCap, lane(5, 6, "first"))
			if err := r.d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			r.set.Listed = []ListedRecord{lane(5, 6, "second")}
			if err := r.d.EndWave(context.Background()); err != nil {
				t.Fatal(err)
			}

			m := r.store.get(5).Members[6]
			if m.Status != status || !slices.Equal(m.PatchIDs, []string{"first"}) {
				t.Fatalf("member = %+v, want status %s and the first patch ids kept", m, status)
			}
			if slices.Contains(flushedPatchIDs(r.actor), "second") {
				t.Fatalf("the actor flushed the second patch ids: %v", flushedPatchIDs(r.actor))
			}
		})
	}
}

func TestDriverAdmits_RowRc_AMergedMemberLandsAtMostOnce(t *testing.T) {
	r := enforceRig(MemberMerged, 1, lane(5, 6, "first"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, again := range []string{"first", "second", "third"} {
		r.set.Listed = []ListedRecord{lane(5, 6, again)}
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	landings := 0
	for _, b := range r.actor.flushes {
		for _, m := range b.Members {
			if m.Cycle == 6 {
				landings++
			}
		}
	}
	if landings != 1 {
		t.Fatalf("member 6 reached the actor %d times, want once", landings)
	}
	if got := r.store.get(5).Members[6]; got.Status != MemberMerged || !slices.Equal(got.PatchIDs, []string{"first"}) {
		t.Fatalf("member = %+v, want merged under the first patch ids", got)
	}
}

func TestDriverAdmits_Row10e_AtShadowAPendingMemberDeliveredAgainIsComposedWithTheNewPatchIDs(t *testing.T) {
	r := newDriverRig(5, lane(5, 6, "first"))
	r.d.Flush = FlushPolicy{Cap: 10, MaxAgeMinutes: 600}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if got := flushedPatchIDs(r.actor); !slices.Equal(got, []string{"second"}) {
		t.Fatalf("flushed patch ids = %v, want only the new ones", got)
	}
	if len(rec.Failures) != 0 || rec.Members[6].Status != MemberComposed {
		t.Fatalf("record = %+v, want a clean wave with the member composed", rec)
	}
}

func TestDriverAdmits_Row10d_AtEnforceAPendingMemberDeliveredAgainUnderANewSHALandsOnceAndEjectsNothing(t *testing.T) {
	r := enforceRig(MemberMerged, 10, lane(5, 6, "first"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	retried := lane(5, 6, "first")
	retried.Record.Commits[0].SHA = "c6-retried-under-a-new-sha"
	r.set.Listed = []ListedRecord{retried}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if got := flushedPatchIDs(r.actor); !slices.Equal(got, []string{"first"}) {
		t.Fatalf("flushed patch ids = %v, want the one member once", got)
	}
	if m := rec.Members[6]; len(rec.Failures) != 0 || m.Status != MemberMerged || !slices.Equal(m.PatchIDs, []string{"first"}) {
		t.Fatalf("record = %+v, want a clean wave with the member landed under its first patch ids", rec)
	}
}
