package integration

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	shaBase = "1111111111111111111111111111111111111111"
	shaTip1 = "2222222222222222222222222222222222222222"
	shaTip2 = "3333333333333333333333333333333333333333"
)

var driverNow = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)

type memStore struct {
	mu      sync.Mutex
	records map[int]WaveRecord
	writes  int
	failAt  map[int]error
}

func newMemStore() *memStore {
	return &memStore{records: map[int]WaveRecord{}, failAt: map[int]error{}}
}

func (s *memStore) Read(pool int) (WaveRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[pool]
	return r, ok, nil
}

func (s *memStore) Write(r WaveRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if err := s.failAt[s.writes]; err != nil {
		return err
	}
	s.records[r.Pool] = cloneRecord(r)
	return nil
}

func (s *memStore) Pools() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int
	for p := range s.records {
		out = append(out, p)
	}
	return out
}

func (s *memStore) get(pool int) WaveRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records[pool]
}

type fixedEpoch struct{ n uint64 }

func (e *fixedEpoch) Current() (uint64, error) { return e.n, nil }

type sink struct {
	mu     sync.Mutex
	events []signalcenter.Event
}

func (s *sink) Emit(e signalcenter.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *sink) codes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.events {
		out = append(out, string(e.Code))
	}
	return out
}

type fakeActor struct {
	mu       sync.Mutex
	resumes  int
	flushes  []PendingBatch
	waveEnds int
	flushErr error
	block    func(ctx context.Context)
	status   MemberStatus
}

func (a *fakeActor) Resume(_ context.Context, rec WaveRecord) (WaveRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resumes++
	return rec, nil
}

func (a *fakeActor) Flush(ctx context.Context, b PendingBatch) (FlushResult, error) {
	if a.block != nil {
		a.block(ctx)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.flushErr != nil {
		return FlushResult{}, a.flushErr
	}
	a.flushes = append(a.flushes, b)
	tip := shaTip1
	if b.Index > 1 {
		tip = shaTip2
	}
	status := a.status
	if status == "" {
		status = MemberComposed
	}
	res := FlushResult{Batch: IntegrationBatch{Index: b.Index, Base: b.Base, Branch: PoolBranch(b.Pool, b.Index), Tip: tip}, Members: map[int]MemberResult{}}
	for _, m := range b.Members {
		res.Batch.Members = append(res.Batch.Members, m.Cycle)
		res.Members[m.Cycle] = MemberResult{Status: status}
	}
	return res, nil
}

func (a *fakeActor) WaveEnd(_ context.Context, rec WaveRecord) (WaveRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waveEnds++
	return rec, nil
}

func lane(pool, cycle int, patch string) ListedRecord {
	rec := DeliverableRecord{Wave: strconv.Itoa(pool), Cycle: cycle, Trailer: CycleTrailer(cycle),
		Commits:     []CommitIdentity{{SHA: fmt.Sprintf("c%d-%s", cycle, patch), BaseSHA: shaBase, PatchID: patch}},
		DeliveredAt: driverNow.Add(time.Duration(cycle) * time.Second)}
	return ListedRecord{Path: filepath.Join("root", "integration-"+strconv.Itoa(pool), "deliverable-"+strconv.Itoa(cycle)+".json"), Pool: pool, Record: rec}
}

type driverRig struct {
	d     *Driver
	store *memStore
	actor *fakeActor
	sink  *sink
	set   *RecordSet
	stage string
}

func newDriverRig(pool int, listed ...ListedRecord) *driverRig {
	r := &driverRig{store: newMemStore(), actor: &fakeActor{}, sink: &sink{}, set: &RecordSet{Listed: listed}, stage: policy.FleetIntegrationShadow}
	r.d = &Driver{
		Pool: pool, Holder: 7, ShadowBase: shaBase,
		Stage:   func() (string, error) { return r.stage, nil },
		Actors:  map[string]Actor{policy.FleetIntegrationShadow: r.actor},
		Epoch:   &fixedEpoch{n: 7},
		Store:   r.store,
		Records: func() RecordSet { return *r.set },
		Flush:   FlushPolicy{Cap: 10, MaxAgeMinutes: 600},
		Last:    func(context.Context) (int, error) { return 99, nil },
		Now:     func() time.Time { return driverNow },
		Publish: r.sink,
	}
	return r
}

func TestDriver_EnforceHasNoActorSoTheDriverRefusesToStart(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "p"))
	r.stage = policy.FleetIntegrationEnforce

	err := r.d.Check()
	tickErr := r.d.Tick(context.Background())

	if err == nil || !strings.Contains(err.Error(), `the stage "enforce" has no actor, so the driver does not start`) {
		t.Fatalf("Check = %v, want the refusal", err)
	}
	if tickErr == nil {
		t.Fatal("Tick ran with no actor")
	}
	if got := r.sink.codes(); len(got) != 1 || got[0] != "SHIP_INTEGRATION_DRIVER_FAILED" {
		t.Fatalf("signals = %v, want one SHIP_INTEGRATION_DRIVER_FAILED", got)
	}
	e := r.sink.events[0]
	if e.Fields["step"] != "stage" || e.Severity != signalcenter.SeverityWarn || e.Kind != signalcenter.KindShipWarning {
		t.Errorf("event = %+v", e)
	}
	wantReason := `shadow: the integration driver of pool 5 failed the step stage: stage: the stage "enforce" has no actor, so the driver does not start`
	if e.Reason != wantReason {
		t.Errorf("reason = %q, want %q", e.Reason, wantReason)
	}
}

func TestDriver_AStageReadFailureIsAStageStep(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Stage = func() (string, error) { return "", errors.New("bad policy") }
	if err := r.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "stage: bad policy") {
		t.Fatalf("Tick = %v, want the stage step", err)
	}
}

func TestDriver_ATickAdmitsDeliveredLanesAsPendingAndWritesTheRecordOnce(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"), lane(5, 7, "b"))

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if len(rec.Members) != 2 || rec.Members[6].Status != MemberPending || rec.Members[7].Status != MemberPending {
		t.Fatalf("members = %+v, want two pending", rec.Members)
	}
	if rec.Epoch != 7 || rec.ShadowBase != shaBase || rec.Ended {
		t.Errorf("record = %+v", rec)
	}
	if r.store.writes != 1 {
		t.Errorf("writes = %d, want 1: an unchanged record is not written again", r.store.writes)
	}
	if len(r.actor.flushes) != 0 {
		t.Errorf("flushes = %d, want none before a trigger", len(r.actor.flushes))
	}
	if r.actor.resumes != 1 {
		t.Errorf("resumes = %d, want 1", r.actor.resumes)
	}
}

func TestDriver_WaveEndFlushesInOrderChainsBatchesAndWritesEnded(t *testing.T) {
	t.Parallel()
	risky := lane(5, 9, "r")
	risky.Record.Risk = "high"
	r := newDriverRig(5, lane(5, 7, "b"), lane(5, 6, "a"), risky)

	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(5)
	if !rec.Ended || rec.LastCycle != 99 {
		t.Fatalf("ended=%v last=%d, want ended with the allocator's number", rec.Ended, rec.LastCycle)
	}
	if len(r.actor.flushes) != 2 {
		t.Fatalf("flushes = %d, want the risky member alone and then the rest", len(r.actor.flushes))
	}
	if first := r.actor.flushes[0]; first.Base != shaBase || len(first.Members) != 1 || first.Members[0].Cycle != 9 {
		t.Errorf("first flush = %+v, want the risky cycle 9 on the shadow base", first)
	}
	second := r.actor.flushes[1]
	if second.Base != shaTip1 || second.Index != 2 || second.Members[0].Cycle != 6 || second.Members[1].Cycle != 7 {
		t.Errorf("second flush = %+v, want index 2 on the tip of batch 1 in delivery order", second)
	}
	var triggers []FlushTrigger
	for _, d := range rec.Decisions {
		triggers = append(triggers, d.Trigger)
	}
	if len(triggers) != 2 || triggers[0] != TriggerRiskyAlone || triggers[1] != TriggerWaveEnd {
		t.Errorf("decisions = %v, want risky_alone then wave_end", triggers)
	}
	for _, c := range []int{6, 7, 9} {
		if rec.Members[c].Status != MemberComposed {
			t.Errorf("member %d = %+v", c, rec.Members[c])
		}
	}
	if got := r.sink.codes(); len(got) != 2 || got[0] != "SHIP_INTEGRATION_FLUSHED" {
		t.Errorf("signals = %v, want two FLUSHED", got)
	}
	e := r.sink.events[0]
	if e.Fields["pr"] != "0" || e.Fields["shadow"] != "true" || e.Fields["trigger"] != "risky_alone" || e.Fields["batch"] != "integration/p5-b1" || e.Fields["pool"] != "5" {
		t.Errorf("flushed fields = %v", e.Fields)
	}
	wantReason := "shadow: pool 5 batch 1 flushed on risky_alone with no PR"
	if e.Reason != wantReason || e.Severity != signalcenter.SeverityInfo || e.Kind != signalcenter.KindShipDelivered {
		t.Errorf("flushed event = %+v", e)
	}
}

func TestDriver_AWaveEndWithNothingDeliveredStillWritesTheWaveEndDecision(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := r.store.get(5)
	if !rec.Ended || len(rec.Decisions) != 1 || rec.Decisions[0].Trigger != TriggerWaveEnd || r.actor.waveEnds != 1 {
		t.Fatalf("record = %+v waveEnds=%d", rec, r.actor.waveEnds)
	}
}

func TestDriver_AFullBatchFlushesBeforeTheWaveEnds(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"), lane(5, 7, "b"))
	r.d.Flush = FlushPolicy{Cap: 2, MaxAgeMinutes: 600}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := r.store.get(5)
	if len(rec.Batches) != 1 || rec.Decisions[0].Trigger != TriggerCapReached || rec.Ended {
		t.Fatalf("record = %+v", rec)
	}
}

func TestDriver_AStaleEpochEndsTheDriverAndWritesNothing(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.d.Epoch = &fixedEpoch{n: 8}

	err := r.d.Tick(context.Background())
	again := r.d.EndWave(context.Background())

	if !errors.Is(err, ErrStaleEpoch) || !errors.Is(again, ErrStaleEpoch) {
		t.Fatalf("errors = %v, %v, want ErrStaleEpoch twice", err, again)
	}
	if r.store.writes != 0 {
		t.Errorf("writes = %d, want none: the holder is no longer the driver", r.store.writes)
	}
	if got := r.sink.codes(); len(got) != 1 {
		t.Errorf("signals = %v, want the failure once", got)
	}
}

func TestDriver_ARecordOfALaterEpochEndsTheDriverAndAnOlderOneIsAdopted(t *testing.T) {
	t.Parallel()
	later := newDriverRig(5)
	later.store.records[5] = WaveRecord{Pool: 5, Epoch: 9, ShadowBase: shaBase}
	if err := later.d.Tick(context.Background()); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("later epoch = %v, want ErrStaleEpoch", err)
	}
	older := newDriverRig(5)
	older.store.records[5] = WaveRecord{Pool: 5, Epoch: 3, ShadowBase: shaBase, Failures: []DriverFailure{{Step: "x", Error: "kept"}}}
	if err := older.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := older.store.get(5); got.Epoch != 7 || len(got.Failures) != 1 {
		t.Errorf("record = %+v, want epoch 7 and the old failure kept", got)
	}
}

func TestDriver_AnEpochReadFailureIsAnEpochStepAndTheDriverGoesOn(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Epoch = epochFunc(func() (uint64, error) { return 0, errors.New("disk") })
	if err := r.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "epoch: disk") {
		t.Fatalf("Tick = %v", err)
	}
	r.d.Epoch = &fixedEpoch{n: 7}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatalf("the next tick = %v, want it to run", err)
	}
}

type epochFunc func() (uint64, error)

func (f epochFunc) Current() (uint64, error) { return f() }

func TestDriver_AFailedWriteIsKeptInFailuresAndTheNextWriteCarriesIt(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.store.failAt[1] = errors.New("disk full")

	if err := r.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "write: disk full") {
		t.Fatalf("first Tick = %v, want the write failure", err)
	}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick = %v", err)
	}

	rec := r.store.get(5)
	if len(rec.Failures) == 0 || rec.Failures[0].Step != "write" || !strings.Contains(rec.Failures[0].Error, "disk full") {
		t.Fatalf("failures = %+v, want the failed write kept in the record", rec.Failures)
	}
}

func TestDriver_ABadRecordFileIsFailedOnceAndBlocksNoOtherMember(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.set.Failures = []RecordFailure{{Path: "root/integration-5/deliverable-9.json", Pool: 5, Scope: ScopePool, Err: errors.New("old schema")}}

	for range 3 {
		if err := r.d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	rec := r.store.get(5)
	if len(rec.Failures) != 1 || rec.Failures[0].Step != "record" || !strings.Contains(rec.Failures[0].Error, "old schema") {
		t.Fatalf("failures = %+v, want the bad file once", rec.Failures)
	}
	if rec.Members[6].Status != MemberPending {
		t.Errorf("member 6 = %+v, want it admitted", rec.Members[6])
	}
}

func TestDriver_AMemberCrossesPoolsWhileItsLatestHolderLeftItPending(t *testing.T) {
	t.Parallel()
	crossing := lane(5, 6, "a")
	r := newDriverRig(6, crossing, lane(2, 3, "old"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberPending, PatchIDs: []string{"a"}}}}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := r.store.get(6)
	if rec.Members[6].Status != MemberPending {
		t.Errorf("member 6 = %+v, want the held member adopted", rec.Members[6])
	}
	if _, ok := rec.Members[3]; ok {
		t.Error("a historical record with no wave record joined the pool")
	}
}

func TestDriver_AMemberThatAnEarlierPoolFinishedOrAFutureOneOwnsIsNotAdopted(t *testing.T) {
	t.Parallel()
	r := newDriverRig(7, lane(5, 6, "a"), lane(9, 8, "z"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberPending, PatchIDs: []string{"a"}}}}
	r.store.records[6] = WaveRecord{Pool: 6, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberComposed, PatchIDs: []string{"a"}}}}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if rec := r.store.get(7); len(rec.Members) != 0 {
		t.Fatalf("members = %+v, want none: pool 6 composed cycle 6 and pool 9 is later", rec.Members)
	}
}

func TestDriver_TheSameCycleWithOtherPatchesIsANewMemberOfTheCurrentPool(t *testing.T) {
	t.Parallel()
	r := newDriverRig(8, lane(5, 6, "first"), lane(8, 6, "second"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberComposed, PatchIDs: []string{"first"}}}}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if m := r.store.get(8).Members[6]; m.Status != MemberPending || len(m.PatchIDs) != 1 || m.PatchIDs[0] != "second" {
		t.Fatalf("member 6 = %+v, want a pending member with the second patch", m)
	}
}

func TestDriver_AFlushFailureIsRecordedAndTheWaveDoesNotEnd(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	r.actor.flushErr = errors.New("git broke")

	err := r.d.EndWave(context.Background())

	if err == nil || !strings.Contains(err.Error(), "flush: git broke") {
		t.Fatalf("Tick = %v", err)
	}
	rec := r.store.get(5)
	if rec.Ended || len(rec.Failures) != 1 || rec.Failures[0].Step != "flush" || rec.Members[6].Status != MemberPending {
		t.Fatalf("record = %+v, want a flush failure, a pending member and no Ended", rec)
	}
}

func TestDriver_ALastCycleFailureIsRecordedOnAnEndedRecord(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Last = func(context.Context) (int, error) { return 0, errors.New("no state") }
	if err := r.d.EndWave(context.Background()); err == nil || !strings.Contains(err.Error(), "last cycle: no state") {
		t.Fatalf("Tick = %v", err)
	}
	rec := r.store.get(5)
	if !rec.Ended || rec.LastCycle != 0 || len(rec.Failures) != 1 {
		t.Fatalf("record = %+v", rec)
	}
	r2 := newDriverRig(5)
	r2.d.Last = nil
	if err := r2.d.EndWave(context.Background()); err != nil || r2.store.get(5).LastCycle != 0 {
		t.Fatalf("no Last port: %v", err)
	}
}

func TestDriver_StartupFailuresAndAMissingBaseAreRecorded(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Startup = []DriverFailure{{Step: "base", Error: "fetch failed", At: driverNow}}
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f := r.store.get(5).Failures; len(f) != 1 || f[0].Error != "fetch failed" {
		t.Fatalf("failures = %+v", f)
	}
	none := newDriverRig(5)
	none.d.ShadowBase = ""
	if err := none.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "no shadow base") {
		t.Fatalf("Tick = %v, want the missing base", err)
	}
	if none.store.writes != 0 {
		t.Error("a record was written without a base")
	}
}

func TestDriver_AStageFlipAtATickChangesTheActor(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "a"))
	other := &fakeActor{}
	r.d.Actors["other"] = other
	if err := r.d.EndWave(context.Background()); err != nil {
		t.Fatal(err)
	}
	r2 := newDriverRig(5, lane(5, 6, "a"))
	r2.d.Actors["other"] = other
	r2.stage = "other"
	if err := r2.d.EndWave(context.Background()); err != nil || other.waveEnds != 1 || r2.actor.waveEnds != 0 {
		t.Fatalf("err=%v other=%d shadow=%d", err, other.waveEnds, r2.actor.waveEnds)
	}
}

func TestDriver_AReadFailureOfTheRecordIsAReadStep(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Store = readFails{r.store}
	if err := r.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "read: torn") {
		t.Fatalf("Tick = %v", err)
	}
}

type readFails struct{ *memStore }

func (readFails) Read(int) (WaveRecord, bool, error) { return WaveRecord{}, false, errors.New("torn") }

func TestFileWaveStore_RoundTripsAndReadsAnAbsentRecordAsNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := FileWaveStore{Root: root}
	if _, found, err := s.Read(5); found || err != nil {
		t.Fatalf("absent = %v, %v", found, err)
	}
	if len(s.Pools()) != 0 {
		t.Fatalf("pools = %v, want none", s.Pools())
	}
	rec := WaveRecord{Pool: 5, Epoch: 2, ShadowBase: shaBase, Ended: true, LastCycle: 9}
	if err := s.Write(rec); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Read(5)
	if err != nil || !found || got.LastCycle != 9 || !got.Ended {
		t.Fatalf("read = %+v, %v, %v", got, found, err)
	}
	if err := s.Write(WaveRecord{Pool: 12, Epoch: 2, ShadowBase: shaBase}); err != nil {
		t.Fatal(err)
	}
	if pools := s.Pools(); len(pools) != 2 || pools[0] != 5 || pools[1] != 12 {
		t.Fatalf("pools = %v, want 5 and 12", pools)
	}
	if err := s.Write(WaveRecord{Pool: 0}); err == nil {
		t.Error("an invalid record was written")
	}
}

func TestStageActors_NamesOnlyTheShadowStage(t *testing.T) {
	t.Parallel()
	got := StageActors(&fakeActor{})
	if _, ok := got[policy.FleetIntegrationShadow]; !ok || len(got) != 1 {
		t.Fatalf("actors = %v", got)
	}
}

func TestFileWaveStore_APoolsListingOfAMalformedRootIsEmpty(t *testing.T) {
	t.Parallel()
	if pools := (FileWaveStore{Root: "["}).Pools(); len(pools) != 0 {
		t.Fatalf("pools = %v, want none", pools)
	}
}

func TestDriver_TheClockAndTheRecordSourceDefaultSafely(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	if d.now().IsZero() {
		t.Error("no clock gave the zero time")
	}
	if got := d.recordSet(); len(got.Listed) != 0 || len(got.Failures) != 0 {
		t.Errorf("recordSet = %+v, want empty", got)
	}
}

type resumeFails struct{ fakeActor }

func (*resumeFails) Resume(context.Context, WaveRecord) (WaveRecord, error) {
	return WaveRecord{}, errors.New("refs locked")
}

type waveEndFails struct{ fakeActor }

func (*waveEndFails) WaveEnd(_ context.Context, rec WaveRecord) (WaveRecord, error) {
	return rec, errors.New("ref delete failed")
}

func TestDriver_AResumeFailureIsAResumeStepAndTheNextTickResumesAgain(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Actors["shadow"] = &resumeFails{}
	if err := r.d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "resume: refs locked") {
		t.Fatalf("Tick = %v", err)
	}
	if r.d.resumed {
		t.Error("a failed resume counted as done")
	}
}

func TestDriver_AWaveEndFailureIsAWaveEndStepAndTheWaveDoesNotEnd(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5)
	r.d.Actors["shadow"] = &waveEndFails{}
	if err := r.d.EndWave(context.Background()); err == nil || !strings.Contains(err.Error(), "wave end: ref delete failed") {
		t.Fatalf("Tick = %v", err)
	}
	if rec := r.store.get(5); rec.Ended {
		t.Fatalf("record = %+v, want no Ended after a failed wave end", rec)
	}
}

func TestDriver_AMemberThatNeverLeavesPendingStopsTheWaveEndLoop(t *testing.T) {
	t.Parallel()
	risky := lane(5, 9, "r")
	risky.Record.Risk = "high"
	r := newDriverRig(5, lane(5, 7, "b"), risky)
	r.actor.status = MemberPending

	err := r.d.EndWave(context.Background())

	if err == nil || !strings.Contains(err.Error(), "did not empty the hold") {
		t.Fatalf("Tick = %v, want the loop bound to fail loudly", err)
	}
	if rec := r.store.get(5); rec.Ended {
		t.Fatal("a wave whose hold never emptied ended")
	}
}

type pairedStore struct {
	*memStore
	pools []int
}

func (p pairedStore) Pools() []int { return p.pools }

func (p pairedStore) Read(pool int) (WaveRecord, bool, error) {
	if pool == 6 {
		return WaveRecord{}, false, errors.New("torn")
	}
	return p.memStore.Read(pool)
}

func TestDriver_AnUnreadableOrMissingWaveRecordBetweenPoolsIsSkippedWhenLookingForTheHolder(t *testing.T) {
	t.Parallel()
	r := newDriverRig(8, lane(5, 6, "a"))
	r.store.records[5] = WaveRecord{Pool: 5, Epoch: 1, ShadowBase: shaBase, Members: map[int]MemberState{6: {Status: MemberPending, PatchIDs: []string{"a"}}}}
	r.d.Store = pairedStore{memStore: r.store, pools: []int{5, 6, 7}}

	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}

	if m := r.store.get(8).Members[6]; m.Status != MemberPending {
		t.Fatalf("member 6 = %+v, want it adopted from pool 5 past a torn record of pool 6 and a missing pool 7", m)
	}
}
