// Package ship implements the native Go commit-and-push phase as a
// core.PhaseRunner.
package ship

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const phaseName = string(core.PhaseShip)

type CmdRunner = sysexec.RunFunc

type Config struct {
	Runner           CmdRunner
	NowFn            func() time.Time
	Sleep            func(time.Duration)
	PhaseIO          config.Stage
	ManifestGate     string
	RepoContractGate string
	Signals          *signalcenter.Center
}

type Phase struct {
	runner           CmdRunner
	nowFn            func() time.Time
	sleep            func(time.Duration)
	phaseIO          config.Stage
	manifestGate     string
	repoContractGate string
	signals          *signalcenter.Center
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{runner: c.Runner, nowFn: nowFn, sleep: c.Sleep, phaseIO: c.PhaseIO, manifestGate: c.ManifestGate, repoContractGate: c.RepoContractGate, signals: c.Signals}
}

func (p *Phase) Name() string { return phaseName }

func (p *Phase) signalsWired() bool { return p.signals != nil }

func defaultCommitMessage(req core.PhaseRequest) string {
	if core.DocumentCycle(req.Workspace) {
		if ids := core.BoundTaskIDs(req.Workspace); len(ids) > 0 {
			msg := fmt.Sprintf("solution(%s): evolve-cycle %d", ids[0], req.Cycle)
			if len(ids) > 1 {
				msg += " (also " + strings.Join(ids[1:], ", ") + ")"
			}
			return msg
		}
	}
	if req.GoalHash != "" {
		return fmt.Sprintf("evolve-cycle %d: goal=%s", req.Cycle, req.GoalHash)
	}
	return fmt.Sprintf("evolve-cycle %d", req.Cycle)
}

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.runner == nil {
		return core.PhaseResponse{}, fmt.Errorf("ship: runner required")
	}
	msg := req.Context["commit_message"]
	if req.Input.Active() {
		msg = req.Input.CycleInputs().CommitMessage()
	}
	if msg == "" {
		msg = defaultCommitMessage(req)
	}

	return p.runNative(ctx, req, msg, start)
}

func NewWithDefaultRunner() *Phase {
	return NewWithDefaultRunnerStage(config.StageOff)
}

func NewWithDefaultRunnerStage(stage config.Stage) *Phase {
	return New(Config{Runner: sysexec.DefaultRunner, PhaseIO: stage})
}

func (p *Phase) shipOptions(req core.PhaseRequest, msg string) Options {
	return Options{
		Class:                           ClassCycle,
		CommitMessage:                   msg,
		ProjectRoot:                     req.ProjectRoot,
		WorkspacePath:                   req.Workspace,
		RunID:                           req.RunID,
		CycleID:                         req.Cycle,
		AuditRound:                      req.AuditRound,
		ActiveWorktree:                  req.Worktree,
		WorktreeBaseSHA:                 req.WorktreeBaseSHA,
		ExplanationDocumentationVersion: req.ExplanationDocumentationVersion,
		BuildExplanation:                req.BuildExplanation,
		RequireBuildExplanationHandoff:  req.ExplanationDocumentationVersion != 0,
		PluginRoot:                      req.Env["EVOLVE_PLUGIN_ROOT"],
		Env:                             req.Env,
		PhaseIO:                         p.phaseIO,
		ManifestGate:                    p.manifestGate,
		Runner:                          p.runner,
		Sleep:                           p.sleep,
		Signals:                         p.signals,
	}
}

func (p *Phase) runNative(ctx context.Context, req core.PhaseRequest, msg string, start time.Time) (core.PhaseResponse, error) {
	opts := p.shipOptions(req, msg)
	res, resumed, err := resumeFirst(ctx, &opts)
	if !resumed && err == nil {
		if gerr := p.runRepoContractGate(ctx, req, &opts); gerr != nil {
			return core.PhaseResponse{}, gerr
		}
		res, err = runStages(ctx, &opts, res)
	}
	resp, rerr := respond(req, res, err)
	resp.DurationMS = p.nowFn().Sub(start).Milliseconds()
	return resp, rerr
}

func (p *Phase) runRepoContractGate(ctx context.Context, req core.PhaseRequest, opts *Options) error {
	gateRoot, baseRef, err := repoContractGateRoot(opts)
	if err != nil {
		return fmt.Errorf("ship repo-contract gate: %w", err)
	}
	if err := runRepoContractGateAt(ctx, p.repoContractGate, gateRoot, baseRef, req.Workspace, os.Stderr, backstopFlakeSignal(p.signals, req.Cycle)); err != nil {
		return fmt.Errorf("ship repo-contract gate: %w", err)
	}
	return nil
}

func respond(req core.PhaseRequest, res RunResult, err error) (core.PhaseResponse, error) {
	resp := core.PhaseResponse{Phase: phaseName, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace}
	switch {
	case err != nil:
		resp.Signals = failureSignals(err, res)
		resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: err.Error()}}
		return resp, fmt.Errorf("ship: native: %w", err)
	case res.ExitCode != ExitOK:
		resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: strings.Join(res.Logs, "\n")}}
		return resp, fmt.Errorf("ship: native exit=%d", res.ExitCode)
	}
	resp.Verdict, resp.NextPhase, resp.CommitSHA = core.VerdictPASS, string(core.PhaseRetro), res.CommitSHA
	if res.RepairAttempted != "" {
		resp.Signals = map[string]any{}
		addRepairSignals(resp.Signals, res)
	}
	return resp, nil
}

func failureSignals(err error, res RunResult) map[string]any {
	signals := map[string]any{}
	if se, ok := core.AsShipError(err); ok {
		signals["ship.error_code"] = string(se.Code)
		signals["ship.error_class"] = string(se.Class)
		signals["ship.error_stage"] = string(se.Stage)
		signals["ship.debug"] = se.DebugString()
	}
	addRepairSignals(signals, res)
	return signals
}

func addRepairSignals(signals map[string]any, res RunResult) {
	if res.RepairAttempted != "" {
		signals["ship.repair_attempted"] = res.RepairAttempted
		signals["ship.repair_outcome"] = res.RepairOutcome
	}
}

func init() {
	registry.Register(string(core.PhaseShip), func(_ core.PhaseRequest) core.PhaseRunner {
		return NewWithDefaultRunner()
	})
}

func repoContractGateRoot(opts *Options) (root, baseRef string, err error) {
	tree, fromWorktree, err := landingTree(opts)
	if err != nil {
		return "", "", err
	}
	if fromWorktree && opts.WorktreeBaseSHA != "" {
		return tree, opts.WorktreeBaseSHA, nil
	}
	return tree, "HEAD", nil
}
