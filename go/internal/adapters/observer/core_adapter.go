package observer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// watcherExitTimeout bounds the cancel's wait for the watcher, which notices
// cancellation only once per PollS.
const watcherExitTimeout = 10 * time.Second

// originAdapterStart is the single spelling of the origin on the adapter's faults.
const originAdapterStart = "CoreAdapter.Start"

// CoreAdapter implements core.Observer by running one Observer.Watch goroutine
// per Start. The zero value uses the compiled policy defaults.
type CoreAdapter struct {
	// Sink, when set, replaces the <workspace>/<phase>-observer-events.ndjson file.
	Sink io.Writer

	// RecoveryStage is the phase-recovery stage; only enforce turns the live
	// channel producer on, and empty resolves to shadow.
	// See ADR-0044.
	RecoveryStage string

	Config policy.ObserverPolicy

	// Signals yields the Signal Center for the adapter's own faults. It is read
	// at every use; a nil Center is the Null Object.
	Signals func() *signalcenter.Center
}

// NewCoreAdapter returns a CoreAdapter using the given observer policy, or the compiled defaults.
func NewCoreAdapter(config ...policy.ObserverPolicy) *CoreAdapter {
	a := &CoreAdapter{}
	if len(config) > 0 {
		a.Config = config[0]
	}
	return a
}

// SignalsWired reports whether the adapter currently reaches a Center.
func (a *CoreAdapter) SignalsWired() bool { return a.reporter().Wired() }

func (a *CoreAdapter) reporter() *observerengine.Reporter {
	return observerengine.NewReporter(a.Signals)
}

// Start implements core.Observer. The caller must call the returned cancel when
// the phase ends; it is idempotent and waits up to watcherExitTimeout so the
// events sink is flushed before the next phase starts.
func (a *CoreAdapter) Start(ctx context.Context, phase string, req core.PhaseRequest) func() {
	if req.Workspace == "" || phase == "" {
		// Pre-cycle hooks have no workspace, so there is nothing to watch.
		return func() {}
	}
	p := observerengine.PathsFor(req.Workspace, phase)

	sink := a.Sink
	var sinkCloser io.Closer
	if sink == nil {
		// Best effort: the orchestrator has normally created the workspace already.
		_ = os.MkdirAll(req.Workspace, 0o755)
		f, err := os.OpenFile(p.Events, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			// Observer failure never blocks the phase; it runs unobserved.
			// See ADR-0030.
			a.reporter().Report(originAdapterStart, req.Cycle, phase, observerengine.CodeEventsSinkOpenFailed,
				err.Error()+" (phase runs unobserved)", map[string]string{"step": "open_sink", "path": p.Events})
			return func() {}
		}
		sink = f
		sinkCloser = f
	}

	resolved := policy.Policy{Observer: &a.Config}.ObserverConfig()
	cfg := Config{
		StallS:       time.Duration(*resolved.StallS) * time.Second,
		PollS:        time.Duration(*resolved.PollS) * time.Second,
		Cycle:        req.Cycle,
		Phase:        phase,
		Agent:        phase, // the runner names the agent after the phase
		StdoutLog:    p.Stdout,
		WorkspaceDir: req.Workspace,
		// Pane hash covers tmux drivers and CPU time covers headless ones; either proves life.
		LivenessProbe: anyProbe(
			newTmuxPaneProbe(req.Cycle, phase, req.RunID, nil),
			newProcessCPUProbe(core.BridgePIDFile(p.Stdout), nil),
		),
	}
	// This path sends no soft-stall nudges: observerengine owns that logic and is
	// not folded in here yet. See docs/architecture/decomposition/12-phaseobserver.md.
	obs := New(cfg, sink)

	watchCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = obs.Watch(watchCtx) // ctx.Err() or nil; neither is a fault
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

// watchHandles is what a running phase's watcher leaves for its cancel.
type watchHandles struct {
	cancel     func()
	prodCancel func()
	obs        *Observer
	wg         *sync.WaitGroup
	sinkCloser io.Closer
}

// finishFn returns Start's idempotent cancel. It stops the watcher and the
// producer and closes the sink only if both exit within timeout. On timeout the
// goroutine and its sink fd leak on purpose, because closing would race the
// watcher's writes, and the leak is signaled.
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

// closeSinkAfterWait closes closer only if done fires within timeout, never on
// the timeout arm, where a leaked watcher may still be writing. A nil closer is
// a no-op. It reports whether done fired in time.
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

// channelSourcePaths returns the files the channel producer tails. A tmux driver
// streams to <phase>-pane.live and <phase>-breadcrumbs.live; a headless driver
// gets empty strings, so the producer keeps its stdout/stderr log defaults.
// See ADR-0037.
func (a *CoreAdapter) channelSourcePaths(req core.PhaseRequest, phase string) (stdout, stderr string) {
	if !bridge.IsTmuxDriver(a.phaseCLI(req, phase)) {
		return "", ""
	}
	return filepath.Join(req.Workspace, phase+"-pane.live"),
		filepath.Join(req.Workspace, phase+"-breadcrumbs.live")
}

// phaseCLI resolves the phase's CLI from the request env (EVOLVE_<PHASE>_CLI,
// then EVOLVE_CLI), defaulting to claude-tmux. It loads no profiles, so a wrong
// guess degrades only the phase's live feed.
func (a *CoreAdapter) phaseCLI(req core.PhaseRequest, phase string) string {
	look := func(k string) string {
		if v, ok := req.Env[k]; ok && v != "" {
			return v
		}
		return ""
	}
	agentKey := "EVOLVE_" + strings.ToUpper(strings.ReplaceAll(phase, "-", "_")) + "_CLI"
	if v := look(agentKey); v != "" {
		return v
	}
	if v := look("EVOLVE_CLI"); v != "" {
		return v
	}
	return "claude-tmux"
}
