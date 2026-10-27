package observer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

const watcherExitTimeout = 10 * time.Second

const originAdapterStart = "CoreAdapter.Start"

type CoreAdapter struct {
	Sink io.Writer

	RecoveryStage string

	Config policy.ObserverPolicy

	Signals func() *signalcenter.Center

	UsageEvidence usageevidence.Explain

	CLIFor func(phase string) string
}

func NewCoreAdapter(config ...policy.ObserverPolicy) *CoreAdapter {
	a := &CoreAdapter{}
	if len(config) > 0 {
		a.Config = config[0]
	}
	return a
}

func (a *CoreAdapter) SignalsWired() bool { return a.reporter().Wired() }

func (a *CoreAdapter) reporter() *observerengine.Reporter {
	return observerengine.NewReporter(a.Signals)
}

func (a *CoreAdapter) Start(ctx context.Context, phase string, req core.PhaseRequest) func() {
	if req.Workspace == "" || phase == "" {
		return func() {}
	}
	p := observerengine.PathsFor(req.Workspace, phase)

	sink := a.Sink
	var sinkCloser io.Closer
	if sink == nil {
		_ = os.MkdirAll(req.Workspace, 0o755)
		f, err := os.OpenFile(p.Events, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			a.reporter().Report(originAdapterStart, req.Cycle, phase, observerengine.CodeEventsSinkOpenFailed,
				err.Error()+" (phase runs unobserved)", map[string]string{"step": "open_sink", "path": p.Events})
			return func() {}
		}
		sink = f
		sinkCloser = f
	}

	resolved := policy.Policy{Observer: &a.Config}.ObserverConfig()
	cfg := Config{
		StallS:        time.Duration(*resolved.StallS) * time.Second,
		PollS:         time.Duration(*resolved.PollS) * time.Second,
		Cycle:         req.Cycle,
		Phase:         phase,
		Agent:         phase,
		StdoutLog:     p.Stdout,
		WorkspaceDir:  req.Workspace,
		LivenessProbe: newProcessCPUProbe(core.BridgePIDFile(p.Stdout), nil),
		PaneWatch: func() (panewatch.Snapshot, bool, error) {
			return panewatch.ReadLive(req.Workspace, phase, runlease.PIDAlive)
		},
	}
	watchCtx, cancel := context.WithCancel(ctx)
	cfg.OnEvent = a.phaseStallSignal(watchCtx, phaseWatch{req: req, phase: phase, stall: cfg.StallS})
	obs := New(cfg, sink)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = obs.Watch(watchCtx)
	}()

	var prodCancel func()
	chOn := channel.Enabled(channel.ResolveStage(a.RecoveryStage))
	if chOn {
		stdoutPath, stderrPath := a.channelSourcePaths(req, phase)
		p := channel.NewProducer(channel.ProducerConfig{
			Workspace: req.Workspace, Agent: phase, Phase: phase, Cycle: req.Cycle,
			StdoutPath: stdoutPath, StderrPath: stderrPath,
		})
		pctx, pcancel := context.WithCancel(ctx)
		prodCancel = pcancel
		wg.Add(1)
		go func() { defer wg.Done(); _ = p.Run(pctx) }()
	}

	return a.finishFn(req.Cycle, phase, watchHandles{cancel: cancel, prodCancel: prodCancel, obs: obs, wg: &wg, sinkCloser: sinkCloser}, watcherExitTimeout)
}

type watchHandles struct {
	cancel     func()
	prodCancel func()
	obs        *Observer
	wg         *sync.WaitGroup
	sinkCloser io.Closer
}

func (a *CoreAdapter) finishFn(cycle int, phase string, h watchHandles, timeout time.Duration) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			h.cancel()
			if h.prodCancel != nil {
				h.prodCancel()
			}
			_ = h.obs.Stop()
			done := make(chan struct{})
			go func() { h.wg.Wait(); close(done) }()
			if !closeSinkAfterWait(done, timeout, h.sinkCloser) {
				a.reporter().Report(originAdapterStart, cycle, phase, observerengine.CodeWatcherLeaked,
					"watcher for phase="+phase+" didn't exit within "+timeout.String()+"; leaking goroutine and leaving its events-sink fd open on purpose — closing it would race the watcher's writes; the OS reclaims it at process exit (cycle should still complete)",
					map[string]string{"step": "cancel", "timeout_s": strconv.FormatFloat(timeout.Seconds(), 'f', -1, 64)})
			}
		})
	}
}

func closeSinkAfterWait(done <-chan struct{}, timeout time.Duration, closer io.Closer) bool {
	select {
	case <-done:
		if closer != nil {
			_ = closer.Close()
		}
		return true
	case <-time.After(timeout):
		return false
	}
}

func (a *CoreAdapter) channelSourcePaths(req core.PhaseRequest, phase string) (stdout, stderr string) {
	if !bridge.IsTmuxDriver(a.phaseCLI(req, phase)) {
		return "", ""
	}
	return filepath.Join(req.Workspace, phase+"-pane.live"),
		filepath.Join(req.Workspace, phase+"-breadcrumbs.live")
}

func (a *CoreAdapter) phaseCLI(req core.PhaseRequest, phase string) string {
	look := func(k string) string {
		if v, ok := req.Env[k]; ok && v != "" {
			return v
		}
		return ""
	}
	if a.CLIFor != nil {
		if v := a.CLIFor(phase); v != "" {
			return v
		}
	}
	if v := look("EVOLVE_CLI"); v != "" {
		return v
	}
	return policy.SeatDefault(policy.SeatObserver).CLI
}
