package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const hostToolTimeout = 800 * time.Millisecond

type hostSampler struct {
	cfg     hostpressure.Config
	stage   string
	read    func() (hostpressure.Reading, error)
	apply   func(int)
	publish orchestrator.Publish
	now     func() time.Time
	goos    string
	state   hostpressure.State
	record  func([]byte) error
	warn    func(error)
	warned  bool
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
}

func (s *hostSampler) sample() {
	reading, err := s.read()
	state, d := hostpressure.Decide(s.state, reading, err == nil, s.cfg, s.now())
	s.state = state
	if d.Blind {
		s.publish.HostBlind(err, s.goos)
	}
	if err != nil {
		return
	}
	loadPerCore := reading.LoadPerCore()
	s.appendSample(hostSampleLine{TS: s.now().UTC().Format(time.RFC3339), Load1: reading.Load1, CPUs: reading.CPUs, LoadPerCore: loadPerCore,
		MemAvailablePct: reading.MemAvailablePct, SwapUsedMB: reading.SwapUsedMB, Score: d.Score, Cap: d.Old, Decided: d.New, Stage: s.stage})
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
	}
}

func hostSamplesPath(cfg loopConfig) string {
	return hostSamplesFilePath(cfg.EvolveDir, cfg.LogPath)
}

func hostSamplesFilePath(evolveDir, logPath string) string {
	dir := filepath.Join(evolveDir, "logs")
	if logPath != "" {
		dir = filepath.Dir(logPath)
	}
	return filepath.Join(dir, "host-pressure.ndjson")
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
		cfg: cfg, stage: a.Stage, read: read, publish: b.deps.Signals.Emit, now: b.clock, goos: runtime.GOOS,
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
