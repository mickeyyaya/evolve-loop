package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	sentinelFullEvery  = 10
	sentinelBlindAfter = 3
)

type sentinelPorts struct {
	read    func(ctx context.Context, full bool) (hostsentinel.Sample, error)
	release func(ctx context.Context) midWaveOutcome
	ticker  func(time.Duration) (<-chan time.Time, func())
	record  func([]byte) error
}

type sentinelLine struct {
	TS           string `json:"ts"`
	FreeBytes    uint64 `json:"free_bytes"`
	Path         string `json:"path"`
	CacheBytes   int64  `json:"cache_bytes"`
	CacheRead    bool   `json:"cache_read"`
	Servers      int    `json:"servers"`
	Orphans      int    `json:"orphans"`
	CountsRead   bool   `json:"counts_read"`
	Held         bool   `json:"held"`
	HoldStage    string `json:"hold_stage"`
	ReleaseStage string `json:"release_stage"`
}

type hostSentinel struct {
	cfg      hostsentinel.Config
	mode     hostsentinel.Mode
	ports    sentinelPorts
	hold     *fleet.FloorHold
	publish  hostsentinel.Publish
	now      func() time.Time
	warn     func(error)
	state    hostsentinel.State
	ticks    int
	failures int
	blind    bool
	warned   bool
}

func sentinelConfig(p gcpolicy.Policy) hostsentinel.Config {
	return hostsentinel.Config{
		FloorBytes: p.DiskFloorBytes(), WarnBytes: p.DiskWarnBytes(), CacheLimitBytes: p.GoCacheMaxBytes(),
		ServersMax: p.Sentinel.OrphanServersLimit(), OrphansMax: p.Sentinel.OrphanProcessesLimit(),
		Repeat: p.Sentinel.Repeat(), HoldMax: p.Sentinel.HoldMaxDuration(), NoProgressBytes: p.Sentinel.NoProgress(),
	}
}

func sentinelSamplesPath(cfg loopConfig) string {
	return filepath.Join(hostLogDir(cfg), "host-sentinel.ndjson")
}

func stageName(enforce bool) string {
	if enforce {
		return string(gcpolicy.SentinelStageEnforce)
	}
	return string(gcpolicy.SentinelStageShadow)
}

func (s *hostSentinel) observe(ctx context.Context, full, prime bool) {
	sample, err := s.ports.read(ctx, full)
	if err != nil {
		s.fail(err)
		return
	}
	s.failures, s.blind = 0, false
	d, st := hostsentinel.Decide(sample, s.cfg, s.state, s.now())
	s.state = st
	s.publish.Announce(d, sample, s.cfg, s.mode)
	s.applyHold(d)
	if d.Release && s.mode.EnforceRelease {
		s.releaseOrDefer(ctx, prime)
	}
	s.record(sample)
}

func (s *hostSentinel) fail(err error) {
	s.failures++
	if s.failures < sentinelBlindAfter || s.blind {
		return
	}
	s.blind = true
	s.publish.Blind(err.Error())
	s.warn(fmt.Errorf("the host sentinel cannot read the host: %w", err))
}

func (s *hostSentinel) applyHold(d hostsentinel.Decision) {
	if !s.mode.EnforceHold {
		return
	}
	switch {
	case d.HoldChanged:
		s.hold.Set(d.Hold)
	case d.Hold:
		s.hold.Poke()
	}
}

func (s *hostSentinel) releaseOrDefer(ctx context.Context, prime bool) {
	if prime {
		s.state = s.state.DeferRelease()
		return
	}
	s.release(ctx)
}

func (s *hostSentinel) release(ctx context.Context) {
	out := s.ports.release(ctx)
	switch {
	case out.Skipped != "":
		s.publish.ReleaseSkipped(out.Skipped)
	case out.Err != "":
		s.publish.ReleaseSkipped(out.Err)
	default:
		s.publish.Released(out.Freed, out.After.CacheBytes)
		s.state = s.state.AfterRelease(s.cfg, out.Freed, out.After)
	}
}

func (s *hostSentinel) record(sample hostsentinel.Sample) {
	line, err := json.Marshal(sentinelLine{
		TS: s.now().UTC().Format(time.RFC3339), FreeBytes: sample.FreeBytes, Path: sample.Path,
		CacheBytes: sample.CacheBytes, CacheRead: sample.CacheRead, Servers: sample.Servers, Orphans: sample.Orphans, CountsRead: sample.CountsRead,
		Held: s.state.Held, HoldStage: stageName(s.mode.EnforceHold), ReleaseStage: stageName(s.mode.EnforceRelease),
	})
	if err == nil {
		err = s.ports.record(append(line, '\n'))
	}
	if err != nil && !s.warned {
		s.warned = true
		s.warn(fmt.Errorf("the host sentinel samples file: %w", err))
	}
}

func (s *hostSentinel) run(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.ticks++
			s.observe(ctx, s.ticks%sentinelFullEvery == 0, false)
		}
	}
}

func (b *loopBatchCoordinator) sentinelWarn(err error) {
	fmt.Fprintf(b.stderr, "[loop] WARN: %v; the loop goes on\n", err)
}

func (b *loopBatchCoordinator) productionSentinelPorts(gcPol gcpolicy.Policy) sentinelPorts {
	root := b.cfg.ProjectRoot
	reader := productionSentinelReader(root, b.cfg.EvolveDir, b.sentinelWarn, sysexec.DefaultRunner)
	run := midWaveRun{
		root: root, budget: gcPol.Sentinel.ReleaseBudget(), lease: takeGCLease,
		newRun: func(ctx context.Context, stdout, stderr io.Writer) gcRun {
			r := newGCRun(ctx, false, stdout, stderr)
			r.cancellable = true
			return r
		},
		free: looppreflight.DiskFreeBytes, cacheDir: gcGoCacheDir, measure: measureTree,
	}
	return sentinelPorts{
		read: reader.read, release: run.run,
		ticker: realHostTicker, record: appendLine(sentinelSamplesPath(b.cfg)),
	}
}

func (b *loopBatchCoordinator) sentinelPortsFor(gcPol gcpolicy.Policy) sentinelPorts {
	ports, prod := b.sentinel, b.productionSentinelPorts(gcPol)
	if ports.read == nil {
		ports.read = prod.read
	}
	if ports.release == nil {
		ports.release = prod.release
	}
	if ports.ticker == nil {
		ports.ticker = prod.ticker
	}
	if ports.record == nil {
		ports.record = prod.record
	}
	return ports
}

func (b *loopBatchCoordinator) startSentinel(ctx context.Context) (*fleet.FloorHold, func()) {
	gcPol := gcBlock(policy.LoadOrDefaults(filepath.Join(b.cfg.EvolveDir, "policy.json"), b.stderr))
	s := &hostSentinel{
		cfg:   sentinelConfig(gcPol),
		mode:  hostsentinel.Mode{EnforceHold: gcPol.Sentinel.HoldEnforced(), EnforceRelease: gcPol.Sentinel.ReleaseEnforced()},
		ports: b.sentinelPortsFor(gcPol), hold: fleet.NewFloorHold(),
		publish: hostsentinel.Publish(b.deps.Signals.Emit), now: b.clock, warn: b.sentinelWarn,
	}
	s.observe(ctx, false, true)
	ticks, stopTicker := s.ports.ticker(gcPol.Sentinel.Interval())
	sentinelCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.run(sentinelCtx, ticks)
	}()
	return s.hold, func() {
		cancel()
		wg.Wait()
		stopTicker()
	}
}
