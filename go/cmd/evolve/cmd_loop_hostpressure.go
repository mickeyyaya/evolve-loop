package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const hostToolTimeout = 800 * time.Millisecond

type hostSampler struct {
	cfg      hostpressure.Config
	stage    string
	read     func() (hostpressure.Reading, error)
	lockWait func() time.Duration
	apply    func(int)
	publish  orchestrator.Publish
	now      func() time.Time
	goos     string
	state    hostpressure.State
	record   func([]byte) error
	warn     func(error)
	warned   bool
}

type hostSampleLine struct {
	TS              string  `json:"ts"`
	Load1           float64 `json:"load1"`
	CPUs            int     `json:"cpus"`
	LoadPerCore     float64 `json:"load_per_core"`
	MemAvailablePct float64 `json:"mem_available_pct"`
	SwapUsedMB      float64 `json:"swap_used_mb"`
	Score           float64 `json:"score"`
	Cap             int     `json:"cap"`
	Decided         int     `json:"decided"`
	Stage           string  `json:"stage"`
	LockWaitMS      int64   `json:"lock_wait_ms"`
	LockWaitHeld    bool    `json:"lock_wait_held"`
}

func (s *hostSampler) sample() {
	reading, err := s.read()
	if err == nil && s.lockWait != nil {
		reading.LockWait = s.lockWait()
	}
	state, d := hostpressure.Decide(s.state, reading, err == nil, s.cfg, s.now())
	s.state = state
	if d.Blind {
		s.publish.HostBlind(err, s.goos)
	}
	if err != nil {
		return
	}
	loadPerCore := reading.LoadPerCore()
	s.appendSample(hostSampleLine{
		TS: s.now().UTC().Format(time.RFC3339), Load1: reading.Load1, CPUs: reading.CPUs, LoadPerCore: loadPerCore,
		MemAvailablePct: reading.MemAvailablePct, SwapUsedMB: reading.SwapUsedMB, Score: d.Score, Cap: d.Old, Decided: d.New, Stage: s.stage,
		LockWaitMS: reading.LockWait.Milliseconds(), LockWaitHeld: d.LockWaitHeld,
	})
	enforce := s.stage == orchestrator.StageEnforce
	changed := s.publish.HostPressureChange(orchestrator.HostPressureEvent{
		Old: d.Old, New: d.New, Sized: s.cfg.Ceiling, Cause: d.Cause, Stage: s.stage, Applied: enforce,
		Load1: reading.Load1, CPUs: reading.CPUs, LoadPerCore: loadPerCore,
		MemAvailablePct: reading.MemAvailablePct, SwapUsedMB: reading.SwapUsedMB, Score: d.Score,
	})
	if changed && enforce {
		s.apply(d.New)
	}
}

func (s *hostSampler) appendSample(line hostSampleLine) {
	raw, err := json.Marshal(line)
	if err == nil {
		err = s.record(append(raw, '\n'))
	}
	if err != nil && !s.warned {
		s.warned = true
		s.warn(err)
	}
}

func (s *hostSampler) run(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.sample()
		}
	}
}

func hostConfig(a policy.AdaptiveConfig, floor, ceiling int) hostpressure.Config {
	return hostpressure.Config{
		LoadHigh: a.LoadHigh, LoadLow: a.LoadLow, MemMinPct: a.MemMinPct, SwapMaxMB: a.SwapMaxMB,
		DecreaseFactor: a.DecreaseFactor, Step: a.Step, SmoothSamples: a.SmoothSamples,
		Floor: min(floor, ceiling), Ceiling: ceiling,
		Cooldown: time.Duration(a.CooldownS) * time.Second, IncreaseCooldown: time.Duration(a.IncreaseCooldownS) * time.Second,
		LockWaitMax: time.Duration(a.LockWaitMaxS) * time.Second,
	}
}

func hostSamplesPath(cfg loopConfig) string {
	return hostSamplesFilePath(cfg.EvolveDir, cfg.LogPath)
}

func hostSamplesFilePath(evolveDir, logPath string) string {
	return filepath.Join(hostLogDir(loopConfig{EvolveDir: evolveDir, LogPath: logPath}), "host-pressure.ndjson")
}

func hostLogDir(cfg loopConfig) string {
	if cfg.LogPath != "" {
		return filepath.Dir(cfg.LogPath)
	}
	return filepath.Join(cfg.EvolveDir, "logs")
}

func appendLine(path string) func([]byte) error {
	return func(line []byte) error {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		_, werr := f.Write(line)
		return errors.Join(werr, f.Close())
	}
}

func hostRun(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	return sysexec.Command(ctx, name, args...).Output()
}

func productionHostReader() func() (hostpressure.Reading, error) {
	return hostpressure.Reader{ReadFile: os.ReadFile, Run: hostRun, CPUs: runtime.NumCPU}.Read
}

func realHostTicker(every time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(every)
	return ticker.C, ticker.Stop
}

type hostSeat struct {
	window *orchestrator.Window
	intake *orchestrator.Intake
}

func (b *loopBatchCoordinator) startHostPressure(ctx context.Context, fc policy.FleetConfig, seat hostSeat) func() {
	a := fc.Orchestrator.Adaptive
	if !a.Enabled || seat.window == nil {
		return func() {}
	}
	read, newTicker := b.hostReader, b.hostTicker
	if read == nil {
		read = productionHostReader()
	}
	if newTicker == nil {
		newTicker = realHostTicker
	}
	cfg := hostConfig(a, fc.Orchestrator.WindowMin, fc.SizedWidth())
	record := b.hostSamples
	if record == nil {
		path := hostSamplesPath(b.cfg)
		record = appendLine(path)
	}
	sampler := &hostSampler{
		record: record,
		warn: func(err error) {
			fmt.Fprintf(b.stderr, "[loop] WARN: orchestrator: host pressure samples file: %v; sampling goes on without it\n", err)
		},
		cfg: cfg, stage: a.Stage, read: read,
		lockWait: func() time.Duration {
			return sumWindowLockWait(b.cfg.EvolveDir, lockWaitWindow(cfg, a), b.clock())
		},
		publish: b.deps.Signals.Emit, now: b.clock, goos: runtime.GOOS,
		state: hostpressure.NewState(cfg.Ceiling),
		apply: func(limit int) {
			seat.window.SetCeiling(limit)
			seat.intake.Kick()
		},
	}
	ticks, stopTicker := newTicker(time.Duration(a.SampleS) * time.Second)
	hostCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sampler.run(hostCtx, ticks)
	}()
	return func() {
		cancel()
		wg.Wait()
		stopTicker()
	}
}

func lockWaitWindow(cfg hostpressure.Config, a policy.AdaptiveConfig) time.Duration {
	sample := time.Duration(max(a.SampleS, 1)) * time.Duration(max(a.SmoothSamples, 1)) * time.Second
	return max(cfg.IncreaseCooldown, sample)
}

func verifyLockWaitMS(e signalcenter.Event) (int64, bool) {
	if e.Code != signalcenter.CodeHostVerifyLockWait {
		return 0, false
	}
	ms, err := strconv.ParseInt(e.Fields["wait_ms"], 10, 64)
	return ms, err == nil && ms > 0
}

func parseRunCycleDir(name string) (int, bool) {
	rest, ok := strings.CutPrefix(name, "cycle-")
	if !ok || rest == "" {
		return 0, false
	}
	id, err := strconv.Atoi(rest)
	return id, err == nil && id >= 0
}

func readRunLockWaitSignals(evolveDir string, minCycle int, visit func(signalcenter.Event)) {
	runsDir := filepath.Join(evolveDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id, ok := parseRunCycleDir(entry.Name())
		if !ok || id <= minCycle {
			continue
		}
		chunk, err := signalcenter.ReadStream(filepath.Join(runsDir, entry.Name(), signalcenter.StreamFileName), 0)
		if err != nil {
			continue
		}
		for _, e := range chunk.Events {
			visit(e)
		}
	}
}

func sumWindowLockWait(evolveDir string, window time.Duration, at time.Time) time.Duration {
	from := at.Add(-window)
	var totalMS int64
	readRunLockWaitSignals(evolveDir, -1, func(e signalcenter.Event) {
		ms, ok := verifyLockWaitMS(e)
		if !ok {
			return
		}
		ts, err := time.Parse(time.RFC3339Nano, e.TS)
		if err != nil || ts.Before(from) || ts.After(at) {
			return
		}
		totalMS += ms
	})
	return time.Duration(totalMS) * time.Millisecond
}
