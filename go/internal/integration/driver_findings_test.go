package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func writeBadRecord(t *testing.T, root, wave string) string {
	t.Helper()
	dir := WaveDir(root, wave)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "deliverable-99.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bad
}

func TestDriver_ABadRecordFileIsAFailureOnlyOfThePoolThatHoldsIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		wave string
		want bool
	}{
		{"the current pool", "5", true},
		{"a named directory", "current", false},
		{"an older pool", "3", false},
		{"a later pool", "9", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePatched(t, root, "5", 6, "c6", "a")
			writeBadRecord(t, root, tc.wave)
			r := newDriverRig(5)
			r.d.Records = func() RecordSet { return ListRecords(root) }

			if err := r.d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}

			rec := r.store.get(5)
			if got := len(rec.Failures) == 1 && rec.Failures[0].Step == "record"; got != tc.want {
				t.Fatalf("failures = %+v, want a record failure: %v", rec.Failures, tc.want)
			}
			if _, ok := rec.Members[6]; !ok {
				t.Errorf("members = %+v, want cycle 6 admitted", rec.Members)
			}
		})
	}
}

func TestDriver_TheSamePoolRedeliveringAPendingCycleUnderNewPatchesReplacesTheMember(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "first"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.set.Listed = []ListedRecord{lane(5, 6, "second")}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if m := rec.Members[6]; m.Status != MemberComposed || len(m.PatchIDs) != 1 || m.PatchIDs[0] != "second" {
		t.Fatalf("member 6 = %+v, want the second patch composed", m)
	}
	if len(r.actor.flushes) != 1 || r.actor.flushes[0].Members[0].Commits[0].PatchID != "second" {
		t.Fatalf("flushes = %+v, want one batch with the second patch", r.actor.flushes)
	}
}

func TestDriver_APendingMemberIsNotReplacedByAnOlderPoolsRecordOfTheSameCycle(t *testing.T) {
	t.Parallel()
	r := newDriverRig(8, lane(8, 6, "second"), lane(5, 6, "first"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberPending, PatchIDs: []string{"first"}}}}

	for range 3 {
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	if m := r.store.get(8).Members[6]; m.Status != MemberPending || m.PatchIDs[0] != "second" {
		t.Fatalf("member 6 = %+v, want the second patch of the current pool", m)
	}
}

type countingStore struct {
	*memStore
	reads map[int]int
}

func (s *countingStore) Read(pool int) (WaveRecord, bool, error) {
	s.memStore.mu.Lock()
	s.reads[pool]++
	s.memStore.mu.Unlock()
	return s.memStore.Read(pool)
}

func (s *countingStore) total() int {
	s.memStore.mu.Lock()
	defer s.memStore.mu.Unlock()
	n := 0
	for _, c := range s.reads {
		n += c
	}
	return n
}

func olderPoolsRig(t *testing.T, pools, perPool int, ended func(pool int) bool) (*driverRig, *countingStore) {
	t.Helper()
	var listed []ListedRecord
	store := newMemStore()
	for p := 1; p <= pools; p++ {
		members := map[int]MemberState{}
		for i := 1; i <= perPool; i++ {
			cycle := p*100 + i
			listed = append(listed, lane(p, cycle, "x"))
			members[cycle] = MemberState{Status: MemberComposed, PatchIDs: []string{"x"}}
		}
		store.records[p] = WaveRecord{Pool: p, Epoch: 1, ShadowBase: shaBase, Ended: ended(p), Members: members}
	}
	r := newDriverRig(pools+1, listed...)
	counting := &countingStore{memStore: store, reads: map[int]int{}}
	r.store, r.d.Store = store, counting
	return r, counting
}

func TestDriver_AnIdleTickReadsEachOlderPoolAtMostOnce(t *testing.T) {
	t.Parallel()
	r, store := olderPoolsRig(t, 40, 5, func(int) bool { return true })

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := store.total(); got > 41 {
		t.Fatalf("store reads = %d, want at most 41 (40 older pools once and the current pool)", got)
	}
	if rec := r.store.get(41); len(rec.Members) != 0 {
		t.Fatalf("members = %+v, want none: every older member composed", rec.Members)
	}
}

func TestDriver_AnEndedPoolWithNoPendingMemberIsNotReadAgainOnTheNextTick(t *testing.T) {
	t.Parallel()
	r, store := olderPoolsRig(t, 40, 5, func(int) bool { return true })
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := store.total()

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := store.total() - before; got != 0 {
		t.Fatalf("the second tick read the store %d times, want 0", got)
	}
}

func TestDriver_APoolThatHasNotEndedIsReadAgainOnTheNextTick(t *testing.T) {
	t.Parallel()
	r, store := olderPoolsRig(t, 4, 2, func(p int) bool { return p != 3 })
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := store.reads[3]

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := store.reads[3] - before; got != 1 {
		t.Fatalf("pool 3 was read %d times on the second tick, want 1", got)
	}
}

func TestDriver_APoolWithAnEndedRecordAndAPendingMemberIsReadAgain(t *testing.T) {
	t.Parallel()
	r, store := olderPoolsRig(t, 3, 1, func(int) bool { return true })
	r.store.records[2] = WaveRecord{Pool: 2, Epoch: 1, ShadowBase: shaBase, Ended: true, Members: map[int]MemberState{201: {Status: MemberPending, PatchIDs: []string{"x"}}}}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := store.reads[2]

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := store.reads[2] - before; got != 1 {
		t.Fatalf("pool 2 was read %d times on the second tick, want 1", got)
	}
}

func TestDriver_APoolWhoseRecordCannotBeReadIsSkippedByTheIndex(t *testing.T) {
	t.Parallel()
	r := newDriverRig(8, lane(5, 6, "a"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberPending, PatchIDs: []string{"a"}}}}
	r.d.Store = pairedStore{memStore: r.store, pools: []int{5, 6}}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if m := r.store.get(8).Members[6]; m.Status != MemberPending {
		t.Fatalf("member 6 = %+v, want it adopted from pool 5", m)
	}
}

func TestKill_AHolderThatGoesStaleAfterItsFirstWriteNeverWritesAgain(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	writes := r.store.writes
	r.d.Epoch = &fixedEpoch{n: 8}
	if err := r.d.Tick(context.Background()); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("Tick = %v", err)
	}
	if r.store.writes != writes {
		t.Fatalf("writes = %d, want %d: a stale holder wrote the record", r.store.writes, writes)
	}
}

type failingFlushActor struct{ *fakeActor }

func (a failingFlushActor) Flush(ctx context.Context, b PendingBatch) (FlushResult, error) {
	res, err := a.fakeActor.Flush(ctx, b)
	res.Failures = []DriverFailure{{Step: "compose", Error: "boom", At: driverNow}}
	return res, err
}

func TestKill_AFailureOfAFlushResultReachesTheRecord(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Actors["shadow"] = failingFlushActor{r.actor}
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f := r.store.get(5).Failures; len(f) != 1 || f[0].Step != "compose" {
		t.Fatalf("failures = %+v, want the compose failure", f)
	}
}

func TestCodeDriverFailed_IsRegisteredToTheShipModuleWithItsDoc(t *testing.T) {
	t.Parallel()
	module, ok := signalcenter.IsRegistered(CodeDriverFailed)
	if !ok || module != signalcenter.ModuleShip {
		t.Fatalf("IsRegistered = %q, %v, want the ship module", module, ok)
	}
	var doc string
	for _, c := range signalcenter.RegisteredCodes()[signalcenter.ModuleShip] {
		if c.Code == CodeDriverFailed {
			doc = c.Doc
		}
	}
	if !strings.Contains(doc, "fields.step") {
		t.Fatalf("doc = %q, want the registered doc of the code", doc)
	}
}
