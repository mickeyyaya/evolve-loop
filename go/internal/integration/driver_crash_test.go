package integration

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type crashRig struct {
	t      *testing.T
	f      composeFixture
	root   string
	evolve string
	store  FileWaveStore
	lanes  []DeliverableRecord
	failW  bool
}

func newCrashRig(t *testing.T, laneCount int) *crashRig {
	t.Helper()
	f := newComposeFixture(t)
	f.g.Git("update-ref", PrivateMainRef, f.base)
	r := &crashRig{t: t, f: f, root: t.TempDir()}
	r.evolve = filepath.Join(r.root, ".evolve")
	r.store = FileWaveStore{Root: r.root}
	for i := range laneCount {
		r.lanes = append(r.lanes, f.lane(t, 6+i, map[string]string{"lane" + strconv.Itoa(6+i) + ".txt": "x\n"}))
	}
	return r
}

type writeGate struct {
	WaveStore
	fail *bool
}

func (w writeGate) Write(rec WaveRecord) error {
	if *w.fail {
		return errors.New("crash before the record")
	}
	return w.WaveStore.Write(rec)
}

func (r *crashRig) driver(visible int, cap int) (*Driver, *EpochHolder) {
	holder := NewEpochHolder(r.evolve)
	claimed, err := holder.Claim()
	if err != nil {
		r.t.Fatal(err)
	}
	var listed []ListedRecord
	for _, l := range r.lanes[:visible] {
		pool, _ := strconv.Atoi(l.Wave)
		listed = append(listed, ListedRecord{Pool: pool, Path: filepath.Join(r.root, "integration-"+l.Wave, "deliverable-"+strconv.Itoa(l.Cycle)+".json"), Record: l})
	}
	records := func() RecordSet { return RecordSet{Listed: listed} }
	actor := ShadowActor{Composer: ShadowComposer(r.f.repo), Records: records, Now: func() time.Time { return driverNow }}
	return &Driver{
		Pool: 135, Holder: claimed, ShadowBase: r.f.base,
		Stage:   func() (string, error) { return policy.FleetIntegrationShadow, nil },
		Actors:  StageActors(actor),
		Epoch:   holder,
		Store:   writeGate{WaveStore: r.store, fail: &r.failW},
		Records: records,
		Flush:   FlushPolicy{Cap: cap, MaxAgeMinutes: 600},
		Now:     func() time.Time { return driverNow },
		Last:    func(context.Context) (int, error) { return 40, nil },
	}, holder
}

func (r *crashRig) refs() string {
	return r.f.g.Git("for-each-ref", "--format=%(refname:short)", "refs/heads/integration")
}

func TestDriverCrash_AfterTheRefBeforeTheRecordTheResumeDropsTheRefAndComposesAgain(t *testing.T) {
	t.Parallel()
	r := newCrashRig(t, 1)
	first, _ := r.driver(1, 1)
	r.failW = true
	if err := first.Tick(context.Background()); err == nil {
		t.Fatal("the crashed tick reported no error")
	}
	if r.refs() != "integration/p135-b1" {
		t.Fatalf("refs after the crash = %q, want the orphan batch ref", r.refs())
	}
	r.failW = false

	second, _ := r.driver(1, 10)
	if err := second.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec, _, _ := r.store.Read(135)
	if len(rec.Batches) != 1 || rec.Members[6].Status != MemberComposed || !rec.Ended {
		t.Fatalf("record = %+v, want one batch, the member composed and the wave ended", rec)
	}
	if r.refs() != "" {
		t.Errorf("refs left = %q", r.refs())
	}
}

func TestDriverCrash_AfterTheRecordANewHolderContinuesTheChainFromTheLastTip(t *testing.T) {
	t.Parallel()
	r := newCrashRig(t, 2)
	first, _ := r.driver(1, 1)
	if err := first.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, _, _ := r.store.Read(135)

	second, _ := r.driver(2, 1)
	if err := second.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec, _, _ := r.store.Read(135)
	if len(rec.Batches) != 2 || !reflect.DeepEqual(rec.Batches[0], before.Batches[0]) || rec.Batches[1].Base != before.Batches[0].Tip || rec.Batches[1].Index != 2 {
		t.Fatalf("batches = %+v, want batch 1 kept and batch 2 on its tip", rec.Batches)
	}
	if rec.Epoch != second.Holder || rec.Epoch == before.Epoch {
		t.Errorf("epoch = %d, want the new holder %d over %d", rec.Epoch, second.Holder, before.Epoch)
	}
	if r.refs() != "" {
		t.Errorf("refs left = %q", r.refs())
	}
}

func TestDriverCrash_ResumeAfterWaveEndChangesNothingButTheHolder(t *testing.T) {
	t.Parallel()
	r := newCrashRig(t, 1)
	first, _ := r.driver(1, 10)
	if err := first.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	ended, _, _ := r.store.Read(135)

	second, _ := r.driver(1, 10)
	for _, step := range []func(context.Context) error{second.Tick, second.EndWave} {
		if err := step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	rec, _, _ := r.store.Read(135)
	if len(rec.Batches) != 1 || !reflect.DeepEqual(rec.Batches[0], ended.Batches[0]) || !rec.Ended || len(rec.Decisions) != len(ended.Decisions) {
		t.Fatalf("record = %+v, want the ended record unchanged", rec)
	}
	if r.refs() != "" {
		t.Errorf("refs left = %q", r.refs())
	}
}

func TestDriverCrash_AStaleHolderCannotWriteOverTheNewHoldersRecord(t *testing.T) {
	t.Parallel()
	r := newCrashRig(t, 1)
	old, _ := r.driver(1, 1)
	current, _ := r.driver(1, 1)
	if err := current.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	want, _, _ := r.store.Read(135)

	err := old.EndWave(context.Background())

	if !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("stale Tick = %v, want ErrStaleEpoch", err)
	}
	got, _, _ := r.store.Read(135)
	if got.Epoch != want.Epoch || got.Ended {
		t.Fatalf("record = %+v, want the new holder's record untouched", got)
	}
}
