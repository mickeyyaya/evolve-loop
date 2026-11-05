package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	driverFinishLimit = 60 * time.Second
	driverTickEvery   = 5 * time.Second
	driverFetchLimit  = 30 * time.Second
	heldBaseRefspec   = "+refs/heads/main:" + integration.PrivateMainRef
	stepHeldBase      = "base"
)

type forkRoute uint8

const (
	routeNone forkRoute = iota
	routePool
	routeWave
	routeRepair
)

type forkOutcome struct {
	route      forkRoute
	mode       string
	results    []fleet.Result
	repaired   bool
	waveConfig policy.FleetConfig
	pace       time.Duration
}

type driverRun struct {
	b   *loopBatchCoordinator
	run *integration.Run
}

func (b *loopBatchCoordinator) driverLimit() time.Duration {
	if b.driverFinishLimit > 0 {
		return b.driverFinishLimit
	}
	return driverFinishLimit
}

func (b *loopBatchCoordinator) driverTickEvery() time.Duration {
	if b.driverTick > 0 {
		return b.driverTick
	}
	return driverTickEvery
}

func (r *driverRun) finish() {
	if r == nil || r.run == nil {
		return
	}
	if r.b.ctx.Err() != nil {
		fmt.Fprintf(r.b.stderr, "[loop] finishing the integration wave record (up to %s); a second interrupt is ignored until it ends\n", r.b.driverLimit())
	}
	if err := r.run.Finish(r.b.driverLimit()); err != nil {
		fmt.Fprintf(r.b.stderr, "[loop] WARN: integration: the wave record did not end cleanly: %v\n", err)
	}
}

func (b *loopBatchCoordinator) startDriver(pool int) *driverRun {
	if pool < 1 {
		return nil
	}
	build := b.newDriver
	if build == nil {
		build = b.productionDriver
	}
	d, err := build(pool)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: integration: the driver of pool %d did not start: %v\n", pool, err)
		return nil
	}
	if d.Check() != nil {
		_ = d.Tick(b.ctx)
		return nil
	}
	return &driverRun{b: b, run: d.Start(b.ctx, b.driverTickEvery())}
}

func (b *loopBatchCoordinator) productionDriver(pool int) (*integration.Driver, error) {
	if b.cfg.Epoch == nil {
		return nil, fmt.Errorf("the loop holds no integration epoch")
	}
	holder, err := b.cfg.Epoch.Claim()
	if err != nil {
		return nil, err
	}
	root := b.cfg.ProjectRoot
	repo := integration.Repo{Dir: root, Git: integration.SysGit()}
	base, startup := heldBase(b.ctx, repo)
	actor := integration.ShadowActor{
		Composer: integration.ShadowComposer(repo),
		Records:  func() integration.RecordSet { return integration.ListRecords(root) },
		Refresh:  func(ctx context.Context) error { return fetchHeldBase(ctx, repo) },
		Publish:  b.deps.Signals,
	}
	return &integration.Driver{
		Pool:       pool,
		Holder:     holder,
		ShadowBase: base,
		Startup:    startup,
		Stage:      b.integrationStage,
		Actors:     integration.StageActors(actor),
		Epoch:      b.cfg.Epoch,
		Store:      integration.FileWaveStore{Root: root},
		Records:    func() integration.RecordSet { return integration.ListRecords(root) },
		Flush:      integration.DefaultFlushPolicy(),
		Last:       b.highestCycle,
		Publish:    b.deps.Signals,
	}, nil
}

func (b *loopBatchCoordinator) integrationStage() (string, error) {
	return policy.LoadOrDefaults(filepath.Join(b.cfg.EvolveDir, "policy.json"), io.Discard).FleetIntegrationStage()
}

func (b *loopBatchCoordinator) highestCycle(ctx context.Context) (int, error) {
	s, err := b.deps.Storage.ReadState(ctx)
	if err != nil {
		return 0, err
	}
	return s.HighestCycleNumber(), nil
}

func fetchHeldBase(ctx context.Context, repo integration.Repo) error {
	ctx, cancel := context.WithTimeout(ctx, driverFetchLimit)
	defer cancel()
	if _, err := repo.Git(ctx, repo.Dir, "", "fetch", "--quiet", "--no-tags", "--refmap=", "origin", heldBaseRefspec); err != nil {
		return fmt.Errorf("fetch the held base: %w", err)
	}
	return nil
}

func heldBase(ctx context.Context, repo integration.Repo) (string, []integration.DriverFailure) {
	var failures []integration.DriverFailure
	if err := fetchHeldBase(ctx, repo); err != nil {
		failures = append(failures, integration.DriverFailure{Step: stepHeldBase, Error: err.Error(), At: time.Now()})
	}
	for _, ref := range []string{integration.PrivateMainRef, "refs/remotes/origin/main"} {
		out, err := repo.Git(ctx, repo.Dir, "", "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
		if err == nil {
			return strings.TrimSpace(out), failures
		}
	}
	return "", failures
}

func (b *loopBatchCoordinator) dispatchFleetIteration(
	iteration int,
	fleetConfig policy.FleetConfig,
	waveBinary string,
	starvation *fleet.StarvationTracker,
) batchDecision {
	if decision, halt := b.diskSpaceHalt(iteration); halt {
		return decision
	}
	b.takeSupply(iteration, fleetConfig)
	out := b.runFork(iteration, fleetConfig, waveBinary)
	return b.settleFork(iteration, fleetConfig, starvation, out)
}

func (b *loopBatchCoordinator) runFork(iteration int, fleetConfig policy.FleetConfig, waveBinary string) forkOutcome {
	pool := b.mintPoolID(fleetConfig)
	run := b.startDriver(pool)
	defer run.finish()
	launch := b.cycleLaunch(waveBinary, pool)
	if loopwave.ShouldRunPool(fleetConfig) {
		if out := b.dispatchPool(iteration, fleetConfig, launch); out.route == routePool {
			return out
		}
	}
	if !shouldRunWave(fleetConfig) {
		return forkOutcome{}
	}
	waveConfig, pace := budgetAwareWaveConfig(b.ctx, fleetConfig, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.deps.Storage, b.stderr)
	recordWaveSized(b.deps.Signals, iteration, fleetConfig.Count, waveConfig.Count)
	out, err := b.wave().Dispatch(b.ctx, b.waveRequest(iteration, waveConfig, launch))
	switch {
	case err != nil:
		return forkOutcome{}
	case out.Ran:
		return forkOutcome{route: routeWave, results: out.Results, waveConfig: waveConfig, pace: pace}
	default:
		return b.repairMinWidth(iteration, fleetConfig, waveConfig, launch)
	}
}

func (b *loopBatchCoordinator) settleFork(iteration int, fleetConfig policy.FleetConfig, starvation *fleet.StarvationTracker, out forkOutcome) batchDecision {
	switch out.route {
	case routePool:
		return b.settlePool(iteration, fleetConfig, out)
	case routeWave:
		return b.settleWave(iteration, fleetConfig, starvation, out)
	case routeRepair:
		b.observeWorkSupply(iteration, fleetConfig, out.waveConfig, repairedLanes(out.repaired), starvation)
		if out.repaired {
			return batchDecision{flow: batchNextIteration}
		}
	}
	return batchDecision{flow: batchProceed}
}

func (b *loopBatchCoordinator) settlePool(iteration int, fleetConfig policy.FleetConfig, out forkOutcome) batchDecision {
	t := laneTally(out.results)
	fmt.Fprintf(b.stderr, "[loop] pool %d: %d/%d lanes ok%s (%s, target=%d)\n",
		iteration, t.OK, t.Total(), t.DeferredSuffix(), out.mode, fleetConfig.Count)
	if decision := b.fleetHaltDecision("pool", iteration, out.results); decision.flow == batchReturn {
		return decision
	}
	applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
	return batchDecision{flow: batchNextIteration}
}

func (b *loopBatchCoordinator) settleWave(iteration int, fleetConfig policy.FleetConfig, starvation *fleet.StarvationTracker, out forkOutcome) batchDecision {
	summary, fields := loopwave.WaveSummary(iteration, laneTally(out.results))
	fmt.Fprintf(b.stderr, "[loop] %s\n", summary)
	emitLoopWave(b.deps.Signals, iteration, "loopBatchCoordinator.settleWave", "", summary, fields)
	if decision := b.fleetHaltDecision("wave", iteration, out.results); decision.flow == batchReturn {
		return decision
	}
	b.observeWorkSupply(iteration, fleetConfig, out.waveConfig, len(out.results), starvation)
	applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
	paceBeforeNextWave(b.ctx, out.pace, b.stderr)
	return batchDecision{flow: batchNextIteration}
}
