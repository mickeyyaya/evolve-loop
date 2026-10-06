package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

type paneWatcher struct {
	tracker   *panewatch.Tracker
	workspace string
	profile   panestream.PaneProfile
	stderr    io.Writer
	prefix    string
	warned    bool
}

func newPaneWatcher(cfg *Config, lp tmuxLaunch, profile panestream.PaneProfile, stderr io.Writer, prefix string) *paneWatcher {
	if cfg.Workspace == "" || cfg.Agent == "" {
		return nil
	}
	identity := panewatch.Snapshot{
		Session: lp.session, Socket: tmuxSocketName(), CLI: lp.name, Agent: cfg.Agent,
		Cycle: cfg.Cycle, RunID: cfg.RunID, Model: dispatchedModel(cfg, lp), WriterPID: os.Getpid(),
	}
	return &paneWatcher{tracker: panewatch.NewTracker(identity), workspace: cfg.Workspace, profile: profile, stderr: stderr, prefix: prefix}
}

func dispatchedModel(cfg *Config, lp tmuxLaunch) string {
	if lp.modelDispatch.model != "" {
		return lp.modelDispatch.model
	}
	return cfg.Model
}

func (p *paneWatcher) observe(pane string, now time.Time) {
	if p == nil || strings.TrimSpace(pane) == "" {
		return
	}
	snap, changed := p.tracker.Observe(panewatch.Frame{
		Hash:       panestream.ProgressHash(pane, p.profile),
		Busy:       panestream.PaneBusy(pane, p.profile),
		TokenLine:  panestream.LastTokenLine(pane, p.profile.TokenLineRegex),
		ModelLabel: panestream.ModelLabel(pane, p.profile.ModelLabelRegex),
	}, now)
	if !changed {
		return
	}
	if err := panewatch.Write(p.workspace, snap); err != nil && !p.warned {
		p.warned = true
		fmt.Fprintf(p.stderr, "%s WARN pane watch: %v (the phase observer falls back to stdout and workspace activity)\n", p.prefix, err)
	}
}

func nowOf(deps Deps) time.Time {
	if deps.Now != nil {
		return deps.Now()
	}
	return time.Now()
}

func (w replWaiter) observeTickPane(state *replWaitState, pane string, captureOK bool) {
	w.channel.observeIdle(pane, state.livenessCenter)
	if captureOK {
		state.paneWatch.observe(pane, nowOf(w.deps))
	}
}

func (s *replWaitState) finishWait(recorder *interaction.Recorder, now func() time.Time) {
	s.recordNudgeOutcome(recorder, now)
	s.paneWatch.close()
}

func (p *paneWatcher) close() {
	if p == nil {
		return
	}
	if err := panewatch.Remove(p.workspace, p.tracker.Agent()); err != nil {
		fmt.Fprintf(p.stderr, "%s WARN pane watch: %v (a stale snapshot stays until the next launch of this agent)\n", p.prefix, err)
	}
}

func (e *Engine) runAttempt(ctx context.Context, req core.BridgeRequest, args []string) launchRun {
	if req.Workspace != "" && req.Agent != "" {
		if err := panewatch.Remove(req.Workspace, req.Agent); err != nil {
			fmt.Fprintf(e.deps.Stderr, "[bridge] WARN pane watch: a previous attempt's snapshot could not be removed: %v\n", err)
		}
	}
	return e.runScoped(ctx, args, req.Env)
}
