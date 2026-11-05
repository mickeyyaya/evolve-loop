package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (b *loopBatchCoordinator) settledPool(iteration int, fc policy.FleetConfig, launch cycleLaunch) (batchDecision, bool) {
	out := b.dispatchPool(iteration, fc, launch)
	if out.route != routePool {
		return batchDecision{}, false
	}
	return b.settleFork(iteration, fc, &fleet.StarvationTracker{}, out), true
}

func (b *loopBatchCoordinator) settledWave(iteration int, fc, wc policy.FleetConfig, results []fleet.Result, pace time.Duration, starvation *fleet.StarvationTracker) batchDecision {
	return b.settleFork(iteration, fc, starvation, forkOutcome{route: routeWave, results: results, waveConfig: wc, pace: pace})
}

func (b *loopBatchCoordinator) settledRepair(iteration int, fc, wc policy.FleetConfig, launch cycleLaunch, starvation *fleet.StarvationTracker) batchDecision {
	return b.settleFork(iteration, fc, starvation, b.repairMinWidth(iteration, fc, wc, launch))
}

const (
	testSha   = "1111111111111111111111111111111111111111"
	waitLimit = 5 * time.Second
)

type stubActor struct {
	mu        sync.Mutex
	waveEnds  int
	flushHold func(ctx context.Context)
}

func (a *stubActor) Resume(_ context.Context, rec integration.WaveRecord) (integration.WaveRecord, error) {
	return rec, nil
}

func (a *stubActor) Flush(ctx context.Context, b integration.PendingBatch) (integration.FlushResult, error) {
	if a.flushHold != nil {
		a.flushHold(ctx)
	}
	return integration.FlushResult{}, errors.New("stub actor flushes nothing")
}

func (a *stubActor) WaveEnd(_ context.Context, rec integration.WaveRecord) (integration.WaveRecord, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waveEnds++
	return rec, nil
}

func (a *stubActor) ends() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.waveEnds
}

type driverEpoch struct{ n uint64 }

func (e driverEpoch) Current() (uint64, error) { return e.n, nil }

type memWaves struct {
	mu   sync.Mutex
	recs map[int]integration.WaveRecord
}

func (m *memWaves) Read(pool int) (integration.WaveRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.recs[pool]
	return r, ok, nil
}

func (m *memWaves) Write(r integration.WaveRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recs == nil {
		m.recs = map[int]integration.WaveRecord{}
	}
	m.recs[r.Pool] = r
	return nil
}

func (m *memWaves) Pools() []int { return nil }

func stubDriver(actor integration.Actor, stage string, waves integration.WaveStore, pub integration.Publisher) func(pool int) (*integration.Driver, error) {
	return func(pool int) (*integration.Driver, error) {
		return &integration.Driver{
			Pool: pool, Holder: 1, ShadowBase: testSha,
			Stage:   func() (string, error) { return stage, nil },
			Actors:  map[string]integration.Actor{policy.FleetIntegrationShadow: actor},
			Epoch:   driverEpoch{n: 1},
			Store:   waves,
			Flush:   integration.DefaultFlushPolicy(),
			Publish: pub,
		}, nil
	}
}

type captureSignals struct {
	mu     sync.Mutex
	events []signalcenter.Event
}

func (c *captureSignals) Emit(e signalcenter.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *captureSignals) codes() []signalcenter.Code {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []signalcenter.Code
	for _, e := range c.events {
		out = append(out, e.Code)
	}
	return out
}

func driverCoordinator(t *testing.T) (*loopBatchCoordinator, *lockedBuffer) {
	t.Helper()
	root := t.TempDir()
	b, console := iscsCoordinator(root, filepath.Join(root, ".evolve"), storage.New(filepath.Join(root, ".evolve")))
	return b, console
}

func TestStartDriver_PoolZeroBuildsNoRunAndAnEmptyFinishIsANoOp(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	b.newDriver = func(int) (*integration.Driver, error) {
		t.Error("the driver factory ran for pool 0")
		return nil, nil
	}
	run := b.startDriver(0)
	run.finish()
	if run != nil || console.String() != "" {
		t.Fatalf("run = %v console = %q, want no run and no output", run, console.String())
	}
}

func TestStartDriver_AFactoryFailureWarnsAndStartsNothing(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	b.newDriver = func(int) (*integration.Driver, error) { return nil, errors.New("no epoch") }
	if run := b.startDriver(7); run != nil {
		t.Fatalf("run = %v, want none", run)
	}
	if want := "[loop] WARN: integration: the driver of pool 7 did not start: no epoch\n"; console.String() != want {
		t.Fatalf("console = %q, want %q", console.String(), want)
	}
}

func TestStartDriver_EnforceHasNoActorSoTheDriverRefusesWithTheSignal(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	sig := &captureSignals{}
	b.newDriver = stubDriver(&stubActor{}, policy.FleetIntegrationEnforce, &memWaves{}, sig)

	run := b.startDriver(7)

	if run != nil {
		t.Fatal("the driver started at enforce")
	}
	if got := sig.codes(); len(got) != 1 || got[0] != integration.CodeDriverFailed {
		t.Fatalf("signals = %v, want one %s", got, integration.CodeDriverFailed)
	}
}

func TestDriverRun_FinishSendsTheWaveEndOnce(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	actor := &stubActor{}
	b.newDriver = stubDriver(actor, policy.FleetIntegrationShadow, &memWaves{}, nil)

	run := b.startDriver(7)
	run.finish()

	if actor.ends() != 1 {
		t.Fatalf("wave ends = %d, want 1; console:\n%s", actor.ends(), console.String())
	}
}

func TestDriverRun_FinishOfAStalledActorReturnsAtTheLimitAndSaysSoOnACancelledLoop(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	entered := make(chan struct{}, 1)
	actor := &stubActor{flushHold: func(ctx context.Context) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
	}}
	waves := &memWaves{}
	waves.recs = map[int]integration.WaveRecord{}
	b.newDriver = func(pool int) (*integration.Driver, error) {
		d, _ := stubDriver(actor, policy.FleetIntegrationShadow, waves, nil)(pool)
		d.Flush = integration.FlushPolicy{Cap: 1, MaxAgeMinutes: 600}
		d.Records = func() integration.RecordSet {
			rec := integration.DeliverableRecord{Wave: "7", Cycle: 8, Trailer: integration.CycleTrailer(8), Commits: []integration.CommitIdentity{{SHA: "c", BaseSHA: "b", PatchID: "p"}}}
			return integration.RecordSet{Listed: []integration.ListedRecord{{Pool: 7, Path: "r/integration-7/deliverable-8.json", Record: rec}}}
		}
		return d, nil
	}
	b.driverFinishLimit, b.driverTick = 50*time.Millisecond, 5*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	run := b.startDriver(7)
	<-entered
	cancel()

	start := time.Now()
	run.finish()

	if elapsed := time.Since(start); elapsed > waitLimit {
		t.Fatalf("finish took %v with a 50ms limit", elapsed)
	}
	if want := "finishing the integration wave record (up to 50ms)"; !strings.Contains(console.String(), want) {
		t.Fatalf("console = %q, want the line %q", console.String(), want)
	}
}

func TestDriverRun_AWaveEndErrorIsAWarningNotAHalt(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	b.newDriver = func(pool int) (*integration.Driver, error) {
		d, _ := stubDriver(&stubActor{}, policy.FleetIntegrationShadow, &failingWaves{}, nil)(pool)
		return d, nil
	}
	b.startDriver(7).finish()
	if !strings.Contains(console.String(), "[loop] WARN: integration: the wave record did not end cleanly: ") {
		t.Fatalf("console = %q", console.String())
	}
}

type failingWaves struct{ memWaves }

func (*failingWaves) Read(int) (integration.WaveRecord, bool, error) {
	return integration.WaveRecord{}, false, errors.New("unreadable")
}

func TestDriverLimitsDefaultToTheSpecAndYieldToTheCoordinatorFields(t *testing.T) {
	t.Parallel()
	b := &loopBatchCoordinator{loadGraph: noModuleGraph}
	if b.driverLimit() != 60*time.Second || b.driverTickEvery() != 5*time.Second {
		t.Fatalf("defaults = %v, %v, want 60s and 5s", b.driverLimit(), b.driverTickEvery())
	}
	b.driverFinishLimit, b.driverTick = time.Second, time.Millisecond
	if b.driverLimit() != time.Second || b.driverTickEvery() != time.Millisecond {
		t.Fatalf("overrides = %v, %v", b.driverLimit(), b.driverTickEvery())
	}
}

func TestProductionDriver_NeedsAnEpochAndHoldsTheWiredPorts(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	if _, err := b.productionDriver(7); err == nil || !strings.Contains(err.Error(), "holds no integration epoch") {
		t.Fatalf("no epoch: err = %v", err)
	}
	b.cfg.Epoch = integration.NewEpochHolder(b.cfg.EvolveDir)
	d, err := b.productionDriver(7)
	if err != nil {
		t.Fatal(err)
	}
	if d.Pool != 7 || d.Holder != 1 || d.Check() != nil {
		t.Fatalf("driver = %+v, check = %v", d, d.Check())
	}
	if stage, err := d.Stage(); err != nil || stage != policy.FleetIntegrationShadow {
		t.Fatalf("stage = %q, %v, want shadow by default", stage, err)
	}
	if set := d.Records(); len(set.Listed) != 0 {
		t.Fatalf("records = %+v", set)
	}
	if d.Flush != integration.DefaultFlushPolicy() {
		t.Errorf("flush policy = %+v", d.Flush)
	}
	if len(d.Startup) != 1 || d.Startup[0].Step != "base" {
		t.Errorf("startup = %+v, want the failed fetch of a repo with no origin recorded", d.Startup)
	}
}

func TestProductionDriver_AnUnwritableEpochDirectoryFails(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	b.cfg.Epoch = integration.NewEpochHolder(filepath.Join(blocker, "sub"))
	if _, err := b.productionDriver(7); err == nil {
		t.Fatal("an epoch claim under a file succeeded")
	}
}

func TestIntegrationStage_ReadsThePolicyAndRefusesAnUnknownStage(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	policyPath := filepath.Join(b.cfg.EvolveDir, "policy.json")
	writeE2File(t, policyPath, `{"fleet":{"integration":{"stage":"enforce"}}}`)
	if stage, err := b.integrationStage(); err != nil || stage != policy.FleetIntegrationEnforce {
		t.Fatalf("stage = %q, %v", stage, err)
	}
	writeE2File(t, policyPath, `{"fleet":{"integration":{"stage":"sideways"}}}`)
	if _, err := b.integrationStage(); err == nil {
		t.Fatal("an unknown stage was accepted")
	}
}

func TestHighestCycle_IsTheAllocatorsNumberAndReportsAStateReadFailure(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	st := b.deps.Storage.(*storage.FilesystemStorage)
	if _, err := core.AllocateCycleNumber(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	if n, err := b.highestCycle(context.Background()); err != nil || n != 1 {
		t.Fatalf("highest = %d, %v, want 1", n, err)
	}
	b.deps.Storage = &erroringReadStorage{}
	if _, err := b.highestCycle(context.Background()); err == nil {
		t.Fatal("a state read failure was hidden")
	}
}

func originAndPlane(t *testing.T) (origin, plane *gittest.Repo) {
	t.Helper()
	origin = gittest.Bare(t)
	plane = gittest.Fixture(t)
	plane.Git("commit", "-q", "--allow-empty", "-m", "base")
	plane.Git("remote", "add", "origin", origin.Dir)
	plane.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	plane.Git("fetch", "-q", "origin")
	return origin, plane
}

func TestHeldBase_FetchesTheLivePrivateRefAndLeavesTheTrackingRefAlone(t *testing.T) {
	t.Parallel()
	origin, plane := originAndPlane(t)
	other := gittest.Clone(t, origin.Dir)
	other.Git("commit", "-q", "--allow-empty", "-m", "moved")
	other.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	moved := strings.TrimSpace(other.Git("rev-parse", "HEAD"))
	tracking := strings.TrimSpace(plane.Git("rev-parse", "refs/remotes/origin/main"))
	repo := integration.Repo{Dir: plane.Dir, Git: integration.SysGit()}

	base, failures := heldBase(context.Background(), repo)

	if base != moved || len(failures) != 0 {
		t.Fatalf("base = %s failures = %+v, want %s and none", base, failures, moved)
	}
	if got := strings.TrimSpace(plane.Git("rev-parse", "refs/remotes/origin/main")); got != tracking {
		t.Fatalf("tracking ref moved from %s to %s", tracking, got)
	}
}

func TestHeldBase_AFailedFetchIsRecordedAndFallsBackToTheLocalRefs(t *testing.T) {
	t.Parallel()
	origin, plane := originAndPlane(t)
	repo := integration.Repo{Dir: plane.Dir, Git: integration.SysGit()}
	plane.Git("remote", "set-url", "origin", filepath.Join(origin.Dir, "gone"))
	tracking := strings.TrimSpace(plane.Git("rev-parse", "refs/remotes/origin/main"))

	base, failures := heldBase(context.Background(), repo)
	if base != tracking || len(failures) != 1 || failures[0].Step != "base" {
		t.Fatalf("base = %s failures = %+v, want the tracking ref and one failure", base, failures)
	}

	plane.Git("update-ref", integration.PrivateMainRef, tracking)
	plane.Git("commit", "-q", "--allow-empty", "-m", "later")
	plane.Git("update-ref", integration.PrivateMainRef, "HEAD")
	held := strings.TrimSpace(plane.Git("rev-parse", "HEAD"))
	if base, _ := heldBase(context.Background(), repo); base != held {
		t.Fatalf("base = %s, want the last held private ref %s", base, held)
	}

	plane.Git("update-ref", "-d", integration.PrivateMainRef)
	plane.Git("update-ref", "-d", "refs/remotes/origin/main")
	if base, failures := heldBase(context.Background(), repo); base != "" || len(failures) != 1 {
		t.Fatalf("base = %q failures = %+v, want no base and the fetch failure", base, failures)
	}
}

func TestSettleFork_ARouteSettlesLikeTheOldDispatchersDid(t *testing.T) {
	t.Parallel()
	b, console := driverCoordinator(t)
	fc := policy.FleetConfig{Count: 2}
	track := &fleet.StarvationTracker{}
	halting := []fleet.Result{{Index: 0, ExitCode: systemFailureHaltExitCode}}

	if d := b.settleFork(1, fc, track, forkOutcome{}); d.flow != batchProceed {
		t.Errorf("no route = %+v, want proceed", d)
	}
	if d := b.settleFork(1, fc, track, forkOutcome{route: routeRepair, repaired: false}); d.flow != batchProceed {
		t.Errorf("an unrepaired wave = %+v, want proceed", d)
	}
	if d := b.settleFork(1, fc, track, forkOutcome{route: routeRepair, repaired: true}); d.flow != batchNextIteration {
		t.Errorf("a repaired wave = %+v, want the next iteration", d)
	}
	if d := b.settleFork(1, fc, track, forkOutcome{route: routePool, mode: "rolling", results: halting}); d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode {
		t.Errorf("a halting pool = %+v, want the halt", d)
	}
	if d := b.settleFork(1, fc, track, forkOutcome{route: routeWave, results: halting}); d.flow != batchReturn {
		t.Errorf("a halting wave = %+v, want the halt", d)
	}
	if !strings.Contains(console.String(), "[loop] pool 1: 0/1 lanes ok") {
		t.Errorf("console = %q, want the pool tally before the halt", console.String())
	}
}
