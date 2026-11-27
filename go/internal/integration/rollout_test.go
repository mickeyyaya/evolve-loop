package integration

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type memChain struct {
	sealed   map[int]string
	sealErr  error
	readErr  error
	sealCtxs []context.Context
	reads    int
}

func newMemChain() *memChain { return &memChain{sealed: map[int]string{}} }

func (c *memChain) Seal(ctx context.Context, pool int, digest string) error {
	c.sealCtxs = append(c.sealCtxs, ctx)
	if c.sealErr != nil {
		return c.sealErr
	}
	c.sealed[pool] = digest
	return nil
}

func (c *memChain) SealedAll(context.Context) (map[int]string, error) {
	c.reads++
	return maps.Clone(c.sealed), c.readErr
}

func cleanRecord(pool int) WaveRecord {
	return WaveRecord{Pool: pool, Epoch: 2, ShadowBase: shaBase, Ended: true, LastCycle: pool + 3,
		Members:   map[int]MemberState{pool + 1: {Status: MemberComposed}},
		Decisions: []FlushRecord{{Trigger: TriggerWaveEnd, At: time.Unix(0, 0).UTC()}}}
}

func sealedStore(t *testing.T, chain ChainPort) (SealedWaveStore, string) {
	t.Helper()
	root := t.TempDir()
	return SealedWaveStore{Inner: FileWaveStore{Root: root}, Root: root, Chain: chain}, root
}

func TestChained_ASealedRecordReadsChainedAndEveryOtherCaseReadsNot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, tc := range map[string]struct {
		prepare func(t *testing.T, s SealedWaveStore, root string, c *memChain)
		want    bool
	}{
		"sealed": {func(t *testing.T, s SealedWaveStore, _ string, _ *memChain) {
			mustWrite(t, s, cleanRecord(1))
		}, true},
		"edited after the seal": {func(t *testing.T, s SealedWaveStore, root string, _ *memChain) {
			mustWrite(t, s, cleanRecord(1))
			raw, _ := os.ReadFile(WaveRecordPath(root, 1))
			if err := os.WriteFile(WaveRecordPath(root, 1), append(raw, ' '), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		"written with no seal": {func(t *testing.T, s SealedWaveStore, _ string, _ *memChain) {
			if err := s.Inner.Write(cleanRecord(1)); err != nil {
				t.Fatal(err)
			}
		}, false},
		"missing record": {func(t *testing.T, _ SealedWaveStore, _ string, c *memChain) {
			c.sealed[1] = "abc"
		}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newMemChain()
			s, root := sealedStore(t, c)
			tc.prepare(t, s, root, c)
			sealed, _ := c.SealedAll(ctx)
			got, err := Chained(sealed, root, 1)
			if err != nil || got != tc.want {
				t.Fatalf("Chained = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func mustWrite(t *testing.T, s WaveStore, rec WaveRecord) {
	t.Helper()
	if err := s.Write(rec); err != nil {
		t.Fatal(err)
	}
}

func TestChained_TheLastSealOfAPoolWins(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s, root := sealedStore(t, c)
	first := cleanRecord(1)
	first.Ended = false
	mustWrite(t, s, first)
	mustWrite(t, s, cleanRecord(1))
	sealed, _ := c.SealedAll(context.Background())
	if got, err := Chained(sealed, root, 1); err != nil || !got {
		t.Fatalf("Chained after the second seal = %v, %v", got, err)
	}
}

func TestChained_AnUnreadableRecordPropagatesItsError(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s, root := sealedStore(t, c)
	mustWrite(t, s, cleanRecord(1))
	if err := os.Chmod(WaveRecordPath(root, 1), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Chained(maps.Clone(c.sealed), root, 1); err == nil && os.Geteuid() != 0 {
		t.Fatal("an unreadable record was dropped")
	}
}

func TestSealedWaveStore_TheSealTakesTheContextOfTheStore(t *testing.T) {
	t.Parallel()
	type key struct{}
	c := newMemChain()
	s, _ := sealedStore(t, c)
	s.Ctx = context.WithValue(context.Background(), key{}, "loop")
	mustWrite(t, s, cleanRecord(1))
	s.Ctx = nil
	mustWrite(t, s, cleanRecord(1))
	if len(c.sealCtxs) != 2 || c.sealCtxs[0].Value(key{}) != "loop" || c.sealCtxs[1] == nil || c.sealCtxs[1].Value(key{}) != nil {
		t.Fatalf("seal contexts = %v", c.sealCtxs)
	}
}

func TestSealedWaveStore_ASealFailureIsAWriteFailureAndAFileFailureSkipsTheSeal(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	sealDown := errors.New("seal down")
	c.sealErr = sealDown
	s, _ := sealedStore(t, c)
	if err := s.Write(cleanRecord(1)); !errors.Is(err, sealDown) {
		t.Fatalf("Write = %v, want the seal error", err)
	}
	noChain, _ := sealedStore(t, nil)
	if err := noChain.Write(cleanRecord(1)); !errors.Is(err, ErrNoChainPort) {
		t.Fatalf("Write with nil chain = %v, want no chain port", err)
	}
	c.sealErr = nil
	bad := cleanRecord(1)
	bad.Epoch = 0
	if err := s.Write(bad); err == nil || len(c.sealed) != 0 {
		t.Fatalf("an invalid record was written or sealed: %v %v", err, c.sealed)
	}
}

func TestPoolVerdict_EachConditionOfCleanFailsOnItsOwn(t *testing.T) {
	t.Parallel()
	good := func() PoolEvidence {
		return PoolEvidence{Found: true, Record: cleanRecord(5), Shipped: []int{6}}
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*PoolEvidence)
		verdict string
		failed  string
	}{
		{"unchanged", func(*PoolEvidence) {}, PoolClean, ""},
		{"no record", func(e *PoolEvidence) { *e = PoolEvidence{} }, PoolNotClean, "no wave record"},
		{"not ended", func(e *PoolEvidence) { e.Record.Ended = false }, PoolNotClean, "has no Ended"},
		{"no wave end", func(e *PoolEvidence) { e.Record.Decisions = nil }, PoolNotClean, "no wave-end decision"},
		{"no last cycle", func(e *PoolEvidence) { e.Record.LastCycle = 0 }, PoolNotClean, "no last cycle"},
		{"shipped without member", func(e *PoolEvidence) { e.Shipped = []int{6, 9} }, PoolNotClean, "cycle 9 shipped and has no member"},
		{"failure", func(e *PoolEvidence) { e.Record.Failures = []DriverFailure{{Step: "flush", Error: "x"}} }, PoolNotClean, "driver failure at flush: x"},
		{"unreadable", func(e *PoolEvidence) { e.ReadFailures = []string{"runs/7"} }, PoolNotClean, "unreadable: runs/7"},
		{"pending at end", func(e *PoolEvidence) { e.Record.Members[8] = MemberState{Status: MemberPending} }, PoolNotClean, "cycle 8 is still pending at wave end"},
		{"no compose", func(e *PoolEvidence) { e.Record.Members = map[int]MemberState{6: {Status: MemberFailed}} }, PoolNoEvidence, ""},
		{"base ahead counts", func(e *PoolEvidence) { e.Record.Members = map[int]MemberState{6: {Status: MemberConflictedBaseAhead}} }, PoolClean, ""},
	} {
		ev := good()
		tc.mutate(&ev)
		verdict, failed := PoolVerdict(ev)
		if verdict != tc.verdict || !strings.Contains(strings.Join(failed, "|"), tc.failed) {
			t.Errorf("%s: verdict %q failed %v", tc.name, verdict, failed)
		}
	}
}

func countOf(root string, c ChainPort, s WaveStore) RolloutCount {
	return RolloutCount{Root: root, Chain: c, Gather: func(pool int) PoolEvidence {
		rec, found, _ := s.Read(pool)
		return PoolEvidence{Found: found, Record: rec, Shipped: []int{pool + 1}}
	}}
}

func TestRolloutCount_CountsOnlyChainedCleanPoolsAndNamesWhyTheOthersFail(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s, root := sealedStore(t, c)
	mustWrite(t, s, cleanRecord(1))
	unclean := cleanRecord(2)
	unclean.Ended = false
	mustWrite(t, s, unclean)
	mustWrite(t, s, cleanRecord(3))
	enforceRec := cleanRecord(4)
	enforceRec.Stage = policy.FleetIntegrationEnforce
	mustWrite(t, s, enforceRec)
	noEvidence := cleanRecord(5)
	noEvidence.Members = map[int]MemberState{6: {Status: MemberFailed}}
	mustWrite(t, s, noEvidence)
	raw, _ := os.ReadFile(WaveRecordPath(root, 3))
	if err := os.WriteFile(WaveRecordPath(root, 3), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(WaveRecordPath(root, 3), 0o644); err != nil {
		t.Fatal(err)
	}
	got := countOf(root, c, s).Count(context.Background(), s.Pools())
	if !slices.Equal(got.Clean, []int{1}) || got.Fault != nil {
		t.Fatalf("clean pools = %v, fault %v, want [1]", got.Clean, got.Fault)
	}
	for pool, want := range map[int]string{2: "not clean", 3: "not chained", 5: PoolNoEvidence} {
		if !strings.Contains(got.Excluded[pool], want) {
			t.Errorf("pool %d excluded as %q, want %q", pool, got.Excluded[pool], want)
		}
	}
	if _, listed := got.Excluded[4]; listed {
		t.Error("an enforce pool is listed as excluded")
	}
}

func TestRolloutCount_ABrokenChainIsAFaultAndNotZeroCleanWaves(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s, root := sealedStore(t, c)
	mustWrite(t, s, cleanRecord(1))
	ledgerBroken := errors.New("ledger broken")
	c.readErr = ledgerBroken
	got := countOf(root, c, s).Count(context.Background(), s.Pools())
	if len(got.Clean) != 0 || !errors.Is(got.Fault, ledgerBroken) {
		t.Fatalf("count = %+v", got)
	}
	nilChain := countOf(root, nil, s).Count(context.Background(), s.Pools())
	if len(nilChain.Clean) != 0 || nilChain.Fault != nil || !strings.Contains(nilChain.Excluded[1], "not chained") {
		t.Fatalf("nil chain count = %+v", nilChain)
	}
}

func TestRolloutCount_AnUnreadableRecordIsExcludedWithItsError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads every file")
	}
	c := newMemChain()
	s, root := sealedStore(t, c)
	mustWrite(t, s, cleanRecord(1))
	if err := os.Chmod(WaveRecordPath(root, 1), 0); err != nil {
		t.Fatal(err)
	}
	count := RolloutCount{Root: root, Chain: c, Gather: func(pool int) PoolEvidence {
		return PoolEvidence{Found: true, Record: cleanRecord(pool), Shipped: []int{pool + 1}}
	}}
	got := count.Count(context.Background(), []int{1})
	if len(got.Clean) != 0 || !strings.Contains(got.Excluded[1], "digest is unreadable") {
		t.Fatalf("count = %+v", got)
	}
}

func TestRolloutCount_ReadsTheChainOnceForAllPools(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s, root := sealedStore(t, c)
	for pool := 1; pool <= 4; pool++ {
		mustWrite(t, s, cleanRecord(pool))
	}
	c.reads = 0
	if got := countOf(root, c, s).Count(context.Background(), s.Pools()); len(got.Clean) != 4 || c.reads != 1 {
		t.Fatalf("clean %v after %d chain reads, want 4 pools and 1 read", got.Clean, c.reads)
	}
}

func TestRolloutError_NamesTheCountAndThePools(t *testing.T) {
	t.Parallel()
	msg := (&RolloutError{Clean: []int{4}, Need: 2}).Error()
	for _, want := range []string{"1/2", "pools 4"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q lacks %q", msg, want)
		}
	}
	if empty := (&RolloutError{Need: 2}).Error(); !strings.Contains(empty, "0/2") || !strings.Contains(empty, "pools none") {
		t.Errorf("empty = %q", empty)
	}
	fault := errors.New("ledger broken")
	broken := &RolloutError{Need: 2, Fault: fault, Excluded: map[int]string{7: "not chained", 3: "not clean"}}
	msg = broken.Error()
	if !errors.Is(broken, fault) || !strings.Contains(msg, "unreadable: ledger broken") || strings.Index(msg, "pool 3 is excluded: not clean") > strings.Index(msg, "pool 7 is excluded: not chained") {
		t.Errorf("broken = %q", msg)
	}
	if line := ReadyLine([]int{3, 5}, 2); line != "enforce ready: 2/2 clean shadow waves (pools 3, 5)" {
		t.Errorf("line = %q", line)
	}
}

func gatedEnforceRig(pool int, clean ...int) *driverRig {
	r := newDriverRig(pool, lane(pool, pool+1, "p"))
	r.stage = policy.FleetIntegrationEnforce
	r.d.Actors[policy.FleetIntegrationEnforce] = &readyActor{}
	r.d.Clean = func() Readiness { return Readiness{Clean: clean} }
	return r
}

func TestDriverGate_EnforceNeedsTwoCleanShadowWaves(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		clean   []int
		refused bool
	}{"none": {nil, true}, "one": {[]int{1}, true}, "two": {[]int{1, 2}, false}} {
		r := gatedEnforceRig(5, tc.clean...)
		err := r.d.Check()
		var refusal *RolloutError
		if tc.refused != errors.As(err, &refusal) {
			t.Errorf("%s: Check = %v, refused %v", name, err, tc.refused)
		}
		if refusal != nil && len(refusal.Clean) != len(tc.clean) {
			t.Errorf("%s: the refusal names %v, want %v", name, refusal.Clean, tc.clean)
		}
	}
	r := gatedEnforceRig(5)
	r.d.Clean = nil
	if err := r.d.Check(); err == nil {
		t.Fatal("a driver with no count port started at enforce")
	}
}

func TestDriverGate_TheCountRunsOncePerDriverAndTheRefusalCarriesTheReasons(t *testing.T) {
	t.Parallel()
	r := gatedEnforceRig(5)
	calls := 0
	fault := errors.New("ledger broken")
	r.d.Clean = func() Readiness {
		calls++
		return Readiness{Fault: fault, Excluded: map[int]string{1: "not chained"}}
	}
	for range 3 {
		var refusal *RolloutError
		if err := r.d.Check(); !errors.As(err, &refusal) || !errors.Is(err, fault) || refusal.Excluded[1] != "not chained" {
			t.Fatalf("Check = %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("the count ran %d times, want 1", calls)
	}
}

func TestDriverGate_ShadowNeedsNoCount(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "p"))
	if err := r.d.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestDriverGate_ALaterPoolWaitsForTheFirstEnforcePool(t *testing.T) {
	t.Parallel()
	r := gatedEnforceRig(4, 1, 2)
	first := cleanRecord(3)
	first.Stage, first.Ended = policy.FleetIntegrationEnforce, false
	r.store.records[3] = first
	var held *PoolHeldError
	if err := r.d.Check(); !errors.As(err, &held) || held.Pool != 4 || held.By != 3 {
		t.Fatalf("Check = %v, want the hold by pool 3", err)
	}
	first.Ended = true
	r.store.records[3] = first
	if err := r.d.Check(); err != nil {
		t.Fatalf("Check after Ended = %v", err)
	}
}

func TestDriverGate_TheFirstEnforcePoolAndShadowPoolsDoNotHoldIt(t *testing.T) {
	t.Parallel()
	r := gatedEnforceRig(3, 1, 2)
	own := cleanRecord(3)
	own.Stage, own.Ended = policy.FleetIntegrationEnforce, false
	r.store.records[3] = own
	shadowOpen := cleanRecord(2)
	shadowOpen.Stage, shadowOpen.Ended = policy.FleetIntegrationShadow, false
	r.store.records[2] = shadowOpen
	if err := r.d.Check(); err != nil {
		t.Fatalf("Check = %v", err)
	}
}

func TestDriver_ARecordCarriesTheStageOfItsWrite(t *testing.T) {
	t.Parallel()
	r := newDriverRig(5, lane(5, 6, "p"))
	if err := r.d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r.store.records[5].Stage; got != policy.FleetIntegrationShadow {
		t.Fatalf("Stage = %q", got)
	}
	bad := cleanRecord(5)
	bad.Stage = "bogus"
	if err := bad.Validate(); err == nil {
		t.Fatal("a bogus stage validates")
	}
}

var errReadFailed = errors.New("read failed")

type failingReadStore struct{ *memStore }

func (failingReadStore) Read(int) (WaveRecord, bool, error) {
	return WaveRecord{}, false, errReadFailed
}

func TestSealedWaveStore_ADelegateThatLeavesNoFileFailsTheDigest(t *testing.T) {
	t.Parallel()
	c := newMemChain()
	s := SealedWaveStore{Inner: newMemStore(), Root: t.TempDir(), Chain: c}
	if err := s.Write(cleanRecord(1)); err == nil || len(c.sealed) != 0 {
		t.Fatalf("Write = %v, sealed %v", err, c.sealed)
	}
	if _, _, err := s.Read(1); err != nil {
		t.Fatalf("Read = %v", err)
	}
	if got := s.Pools(); len(got) != 1 {
		t.Fatalf("Pools = %v", got)
	}
}

func TestDriverGate_AnUnreadableRecordHoldsTheDriverAndTheHoldNamesBothPools(t *testing.T) {
	t.Parallel()
	r := gatedEnforceRig(4, 1, 2)
	r.store.records[3] = cleanRecord(3)
	r.d.Store = failingReadStore{r.store}
	if err := r.d.Check(); !errors.Is(err, errReadFailed) {
		t.Fatalf("Check = %v", err)
	}
	msg := (&PoolHeldError{Pool: 4, By: 3}).Error()
	if !strings.Contains(msg, "pool 4") || !strings.Contains(msg, "pool 3") {
		t.Fatalf("hold message %q", msg)
	}
}

func TestHeldBy_OnlyAnOpenFirstEnforcePoolHoldsAnotherPool(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	store.records[2] = WaveRecord{Pool: 2, Stage: policy.FleetIntegrationShadow}
	store.records[3] = WaveRecord{Pool: 3, Stage: policy.FleetIntegrationEnforce}
	cases := []struct {
		name     string
		pool     int
		ended    bool
		wantBy   int
		wantHeld bool
	}{
		{"a later pool waits for the open first enforce pool", 5, false, 3, true},
		{"the first enforce pool never holds itself", 3, false, 3, false},
		{"an ended first enforce pool holds nothing", 5, true, 3, false},
	}
	for _, c := range cases {
		rec := store.records[3]
		rec.Ended = c.ended
		store.records[3] = rec
		by, held, err := HeldBy(store, c.pool)
		if err != nil || by != c.wantBy || held != c.wantHeld {
			t.Errorf("%s: HeldBy = (%d, %v, %v), want (%d, %v, nil)", c.name, by, held, err, c.wantBy, c.wantHeld)
		}
	}
	shadowOnly := newMemStore()
	shadowOnly.records[2] = WaveRecord{Pool: 2, Stage: policy.FleetIntegrationShadow}
	if _, held, err := HeldBy(shadowOnly, 5); held || err != nil {
		t.Errorf("shadow pools only: held %v err %v, want no hold", held, err)
	}
}

func TestRecordDigest_TracksTheBytesOfTheRecordFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := RecordDigest(root, 7); err == nil {
		t.Fatal("a missing record gave a digest")
	}
	path := WaveRecordPath(root, 7)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"pool":7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := RecordDigest(root, 7)
	if err != nil || len(first) != 64 {
		t.Fatalf("digest %q err %v, want 64 hex characters", first, err)
	}
	again, _ := RecordDigest(root, 7)
	if again != first {
		t.Fatalf("the same bytes gave %q then %q", first, again)
	}
	if err := os.WriteFile(path, []byte(`{"pool":7} `), 0o644); err != nil {
		t.Fatal(err)
	}
	if edited, _ := RecordDigest(root, 7); edited == first {
		t.Fatal("an edited record kept its digest")
	}
}

func TestRolloutNeed_IsTheTwoCleanShadowWavesOfTheOperatorRule(t *testing.T) {
	t.Parallel()
	if RolloutNeed != 2 {
		t.Fatalf("RolloutNeed = %d, want 2 (docs/architecture/integration-driver-enforce.md section 12)", RolloutNeed)
	}
}
