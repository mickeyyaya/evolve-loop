package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type fakeChain struct {
	sealed  map[int]string
	readErr error
}

func (c *fakeChain) Seal(_ context.Context, pool int, digest string) error {
	c.sealed[pool] = digest
	return nil
}

func (c *fakeChain) SealedAll(context.Context) (map[int]string, error) {
	return maps.Clone(c.sealed), c.readErr
}

func seedCleanShadowPools(t *testing.T, b *loopBatchCoordinator, root string, pools ...int) *fakeChain {
	t.Helper()
	chain := &fakeChain{sealed: map[int]string{}}
	store := integration.SealedWaveStore{Inner: integration.FileWaveStore{Root: root}, Root: root, Chain: chain}
	for _, pool := range pools {
		rec := integration.WaveRecord{
			Pool: pool, Epoch: 1, ShadowBase: testSha, Stage: policy.FleetIntegrationShadow,
			Ended: true, LastCycle: pool + 2,
			Members:   map[int]integration.MemberState{pool + 1: {Status: integration.MemberComposed}},
			Decisions: []integration.FlushRecord{{Trigger: integration.TriggerWaveEnd, At: time.Unix(0, 0).UTC()}},
		}
		if err := store.Write(rec); err != nil {
			t.Fatal(err)
		}
	}
	b.newChain = func(string) integration.ChainPort { return chain }
	return chain
}

func (r statusRig) runChained(chain integration.ChainPort, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := statusVerb(integrationEnv{Root: r.root, Chain: chain}, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (r statusRig) sealedPool(t *testing.T, chain *fakeChain, pool int, mutate func(*integration.WaveRecord)) {
	t.Helper()
	r.writePool(t, pool, mutate)
	digest, err := integration.RecordDigest(r.root, pool)
	if err != nil {
		t.Fatal(err)
	}
	chain.sealed[pool] = digest
}

func TestStatusVerb_PrintsTheEnforceReadyCountOfChainedCleanPools(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	chain := &fakeChain{sealed: map[int]string{}}
	r.sealedPool(t, chain, 3, nil)
	r.sealedPool(t, chain, 4, func(rec *integration.WaveRecord) { rec.Ended = false })
	code, stdout, _ := r.runChained(chain, "--pool", "3")
	if code != 0 || !strings.Contains(stdout, "enforce ready: 1/2 clean shadow waves (pools 3)") {
		t.Fatalf("status = %d\n%s", code, stdout)
	}
	if strings.Contains(stdout, "no enforce actor") {
		t.Fatalf("the old note stays:\n%s", stdout)
	}
}

func TestStatusVerb_AnEditedRecordNeverCountsAsClean(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	chain := &fakeChain{sealed: map[int]string{}}
	r.sealedPool(t, chain, 3, nil)
	path := integration.WaveRecordPath(r.root, 3)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := r.runChained(chain, "--pool", "3")
	if code != 0 || !strings.Contains(stdout, "enforce ready: 0/2 clean shadow waves (pools none)") || !strings.Contains(stdout, "verdict: clean") {
		t.Fatalf("status = %d\n%s", code, stdout)
	}
}

func TestStatusVerb_JSONCarriesTheEnforceReadyFieldsAndTwoCleanPoolsListBoth(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	chain := &fakeChain{sealed: map[int]string{}}
	r.sealedPool(t, chain, 3, nil)
	r.sealedPool(t, chain, 5, nil)
	_, stdout, _ := r.runChained(chain, "--pool", "5", "--json")
	var rep integrationStatusReport
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.EnforceInfo.Clean != 2 || rep.EnforceInfo.Need != 2 || len(rep.EnforceInfo.Pools) != 2 || !strings.Contains(rep.Enforce, "(pools 3, 5)") {
		t.Fatalf("report = %+v", rep.EnforceInfo)
	}
}

func TestLedgerChain_ASealedDigestReadsBackAndTheLastSealWins(t *testing.T) {
	t.Parallel()
	chain := ledgerChain{root: ledgerRoot(t)}
	ctx := context.Background()
	if got, err := chain.SealedAll(ctx); err != nil || len(got) != 0 {
		t.Fatalf("an empty ledger reads %v, %v", got, err)
	}
	for _, digest := range []string{"aaaa", "bbbb"} {
		if err := chain.Seal(ctx, 4, digest); err != nil {
			t.Fatalf("Seal %s: %v", digest, err)
		}
	}
	if err := chain.Seal(ctx, 5, "cccc"); err != nil {
		t.Fatal(err)
	}
	got, err := chain.SealedAll(ctx)
	if err != nil || len(got) != 2 || got[4] != "bbbb" || got[5] != "cccc" {
		t.Fatalf("SealedAll = %v, %v, want 4 sealed twice with the second digest and 5 once", got, err)
	}
}

func TestLastDigests_TheHighestSequenceWinsInAnyOrderAndOtherRecordsAreSkipped(t *testing.T) {
	t.Parallel()
	pool := func(id, seq int, digest string) stageevidence.Record {
		return stageevidence.Record{Key: waveRecordKey, Kind: stageevidence.KindDecision, Scope: stageevidence.ScopePool, ID: id, Seq: seq, Fields: map[string]string{chainFieldDigest: digest}}
	}
	other := pool(4, 9, "wrong")
	other.Key = "fleet.integration.stage"
	late := pool(4, 10, "late")
	late.Kind = stageevidence.KindLate
	cycle := pool(4, 11, "cycle")
	cycle.Scope = stageevidence.ScopeCycle
	for name, records := range map[string][]stageevidence.Record{
		"append order":   {pool(4, 1, "first"), pool(4, 3, "last"), pool(4, 2, "middle"), other, late, cycle},
		"reversed order": {cycle, late, other, pool(4, 2, "middle"), pool(4, 3, "last"), pool(4, 1, "first")},
	} {
		if got := lastDigests(records); len(got) != 1 || got[4] != "last" {
			t.Errorf("%s: %v, want pool 4 at last", name, got)
		}
	}
}

func TestLedgerChain_ASealWithAnUnwritableEvolveDirFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (ledgerChain{root: root}).Seal(context.Background(), 4, "aaaa"); err == nil {
		t.Fatal("Seal passed with no evolve directory")
	}
}

func TestLedgerChain_ABrokenLedgerFailsSealedAll(t *testing.T) {
	t.Parallel()
	root := ledgerRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "ledger.jsonl"), []byte("{\"kind\":\"stage-evidence-seal\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chain := ledgerChain{root: root}
	if err := chain.Seal(context.Background(), 4, "aaaa"); err == nil {
		t.Fatal("Seal passed on a broken ledger")
	}
	if _, err := chain.SealedAll(context.Background()); err == nil {
		t.Fatal("SealedAll passed on a broken ledger")
	}
}

func TestStatusVerb_ABrokenChainPrintsTheFaultAndTheExcludedPoolsWithTheirReasons(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	chain := &fakeChain{sealed: map[int]string{}}
	r.sealedPool(t, chain, 3, nil)
	chain.readErr = errors.New("ledger broken")
	_, stdout, _ := r.runChained(chain, "--pool", "3")
	for _, want := range []string{"enforce ready: 0/2", "enforce ready fault: the evidence chain is unreadable: ledger broken"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	chain.readErr = nil
	delete(chain.sealed, 3)
	_, stdout, _ = r.runChained(chain, "--pool", "3", "--json")
	var rep integrationStatusReport
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil || !strings.Contains(rep.EnforceInfo.Excluded[3], "not chained") || rep.EnforceInfo.Fault != "" {
		t.Fatalf("report = %+v, %v", rep.EnforceInfo, err)
	}
	_, text, _ := r.runChained(chain, "--pool", "3")
	if !strings.Contains(text, "enforce ready excludes pool 3: the wave record is not chained") {
		t.Fatalf("stdout lacks the exclusion reason:\n%s", text)
	}
}

func sendTick(t *testing.T, ticks chan<- struct{}, done <-chan forkOutcome) {
	t.Helper()
	select {
	case ticks <- struct{}{}:
	case <-done:
		t.Fatal("runFork returned while the first enforce pool was open")
	case <-time.After(10 * time.Second):
		t.Fatal("the hold loop never waited for a tick")
	}
}

func TestRunFork_HoldsASecondPoolStartUntilTheFirstEnforcePoolEnds(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	writeIntegrationStage(t, b.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	ticks := make(chan struct{})
	b.holdTicks = ticks
	store := &memWaves{recs: map[int]integration.WaveRecord{
		3: {Pool: 3, Epoch: 1, ShadowBase: testSha, Stage: policy.FleetIntegrationEnforce, Ended: false},
	}}
	b.waveStore = store
	var started atomic.Int32
	b.newDriver = func(pool int) (*integration.Driver, error) {
		started.Store(int32(pool))
		return nil, errors.New("stop after mint")
	}
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}

	done := make(chan forkOutcome, 1)
	go func() {
		done <- b.runFork(1, fc, "/nonexistent/evolve")
	}()

	sendTick(t, ticks, done)
	sendTick(t, ticks, done)
	if started.Load() != 0 {
		t.Fatalf("driver started pool %d while pool 3 was open", started.Load())
	}
	if n := strings.Count(console.String(), "holding pool start"); n != 1 || !strings.Contains(console.String(), "stop the loop and end or repair the wave record of pool 3") {
		t.Fatalf("the hold printed %d lines, console %q", n, console.String())
	}

	store.mu.Lock()
	r := store.recs[3]
	r.Ended = true
	store.recs[3] = r
	store.mu.Unlock()

	released := false
	for !released {
		select {
		case ticks <- struct{}{}:
		case <-done:
			released = true
		case <-time.After(10 * time.Second):
			t.Fatal("runFork stayed held after the first enforce pool ended")
		}
	}
	if started.Load() == 0 {
		t.Fatal("runFork did not start a pool after the first enforce pool ended")
	}
}

func TestRunFork_StopsHoldingOnCancelOrUnreadableWaveStore(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	writeIntegrationStage(t, b.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	ticks := make(chan struct{})
	b.holdTicks = ticks
	b.waveStore = &memWaves{recs: map[int]integration.WaveRecord{
		3: {Pool: 3, Epoch: 1, ShadowBase: testSha, Stage: policy.FleetIntegrationEnforce, Ended: false},
	}}
	var started atomic.Int32
	b.newDriver = func(pool int) (*integration.Driver, error) {
		started.Store(int32(pool))
		return nil, errors.New("stop")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	done := make(chan forkOutcome, 1)
	go func() { done <- b.runFork(1, fc, "/nonexistent/evolve") }()
	sendTick(t, ticks, done)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runFork stayed held after the context was cancelled")
	}
	if started.Load() != 0 {
		t.Fatalf("cancelled runFork started pool %d", started.Load())
	}

	b2, console2 := driverCoordinator(t)
	writeIntegrationStage(t, b2.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	b2.waveStore = &failingWaves{memWaves{recs: map[int]integration.WaveRecord{3: {Pool: 3}}}}
	b2.newDriver = func(pool int) (*integration.Driver, error) {
		started.Store(int32(pool))
		return nil, errors.New("stop")
	}
	b2.runFork(1, fc, "/nonexistent/evolve")
	if started.Load() != 0 || !strings.Contains(console2.String(), "could not read the first enforce pool") {
		t.Fatalf("started=%d console=%q", started.Load(), console2.String())
	}

	b3, _ := driverCoordinator(t)
	if !b3.waitFirstEnforcePool(policy.FleetConfig{Count: 1}) {
		t.Fatal("sequential config was held")
	}
	if !b3.waitFirstEnforcePool(fc) {
		t.Fatal("shadow stage was held")
	}
	writeIntegrationStage(t, b3.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	if !b3.waitFirstEnforcePool(fc) {
		t.Fatal("enforce stage with default store and no open pool was held")
	}
}

func ledgerRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for path, body := range map[string]string{"go/go.mod": "module fixture\n\ngo 1.22\n", ".evolve/policy.json": "{}"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLedgerChain_SealsNeverCountAsPromotionDecisionsOfTheStageKey(t *testing.T) {
	t.Parallel()
	root := ledgerRoot(t)
	chain := ledgerChain{root: root}
	for pool := 1; pool <= 3; pool++ {
		if err := chain.Seal(context.Background(), pool, "aaaa"); err != nil {
			t.Fatal(err)
		}
	}
	version, err := newStageEvidenceSite(root, nil, nil).versionOf()("fleet.integration.stage")
	if err != nil {
		t.Fatal(err)
	}
	res, err := stageevidence.Verify(context.Background(), stageLedgerReader(root), "fleet.integration.stage", version, gatecontract.Promotion{MinDecisions: 1}, nil)
	if !errors.Is(err, stageevidence.ErrEmptyChain) || res.Decisions != 0 || len(res.OtherVersions) != 0 || res.Ready {
		t.Fatalf("the stage chain counts %d decisions (other versions %v), ready %v, err %v", res.Decisions, res.OtherVersions, res.Ready, err)
	}
}

func TestRunFork_AShadowStageNeverWaitsForAnOpenEnforceRecord(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	writeIntegrationStage(t, b.cfg.EvolveDir, policy.FleetIntegrationShadow)
	b.driverTick = time.Hour
	b.waveStore = &memWaves{recs: map[int]integration.WaveRecord{
		3: {Pool: 3, Epoch: 1, ShadowBase: testSha, Stage: policy.FleetIntegrationEnforce, Ended: false},
	}}
	var started atomic.Int32
	b.newDriver = func(pool int) (*integration.Driver, error) {
		started.Store(int32(pool))
		return nil, errors.New("stop after mint")
	}
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	done := make(chan forkOutcome, 1)
	go func() { done <- b.runFork(1, fc, "/nonexistent/evolve") }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a shadow stage held the pool start")
	}
	if started.Load() == 0 {
		t.Fatal("the shadow stage minted no pool")
	}
}

func TestWaitHoldTick_TheProductionTimerFiresAndACancelledLoopStopsTheHold(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	b.holdTicks = nil
	b.driverTick = time.Millisecond
	b.ctx = context.Background()
	if !b.waitHoldTick() {
		t.Fatal("the production timer of the hold did not fire")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.ctx = ctx
	b.driverTick = time.Hour
	if b.waitHoldTick() {
		t.Fatal("a cancelled loop kept the hold waiting")
	}
	seamCtx, seamCancel := context.WithCancel(context.Background())
	seamCancel()
	b.ctx = seamCtx
	b.holdTicks = make(chan struct{})
	if b.waitHoldTick() {
		t.Fatal("a cancelled loop kept the seam hold waiting")
	}
}

func TestChainOf_TheDriverChainCarriesTheLoopSignalCenter(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	if b.deps.Signals == nil {
		t.Fatal("the coordinator fixture has no Signal Center")
	}
	chain, ok := b.chainOf(b.cfg.ProjectRoot).(ledgerChain)
	if !ok || chain.signals != b.deps.Signals || chain.root != b.cfg.ProjectRoot {
		t.Fatalf("chainOf = %#v, want a ledgerChain of the project root that carries the loop Signal Center", chain)
	}
}

func TestWaitFirstEnforcePool_ASequentialForkAtEnforceNeverWaits(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	writeIntegrationStage(t, b.cfg.EvolveDir, policy.FleetIntegrationEnforce)
	b.driverTick = time.Hour
	b.waveStore = &memWaves{recs: map[int]integration.WaveRecord{
		3: {Pool: 3, Epoch: 1, ShadowBase: testSha, Stage: policy.FleetIntegrationEnforce},
	}}
	fc := policy.FleetConfig{Count: 1, Concurrency: 1}
	done := make(chan bool, 1)
	go func() { done <- b.waitFirstEnforcePool(fc) }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("a sequential fork at enforce was refused")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a sequential fork waited on the enforce pool hold")
	}
}
