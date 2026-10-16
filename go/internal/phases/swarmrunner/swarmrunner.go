// Package swarmrunner is the orchestrator seam for the swarm harness: a decorator
// that runs a phase across parallel workers when the swarm stage is active.
// See docs/architecture/packages/internal-phases-swarmrunner.md.
package swarmrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type stage int

const (
	stageOff stage = iota
	stageAdvisory
	stageEnforce
)

type Config struct {
	Stage        string
	PortBase     int
	WorktreeBase string
}

type Decorator struct {
	inner       core.PhaseRunner
	bridge      core.Bridge
	mode        swarm.Mode
	concurrency int
	cfg         Config
}

func New(inner core.PhaseRunner, bridge core.Bridge, mode swarm.Mode, cfg Config) *Decorator {
	return &Decorator{inner: inner, bridge: bridge, mode: mode, concurrency: 2, cfg: cfg}
}

func (d *Decorator) Name() string { return d.inner.Name() }

func (d *Decorator) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	st := parseSwarmStage(d.cfg.Stage)
	if st == stageOff {
		return d.inner.Run(ctx, req)
	}
	plan, ok := loadPlan(req.Workspace)
	if !ok {
		return d.inner.Run(ctx, req)
	}
	plan.Mode = d.mode
	if v := swarm.Validate(plan); v.Collapse {
		return d.inner.Run(ctx, req)
	}
	if d.mode == swarm.ModeWriter {
		err := fmt.Errorf("writer swarm is unsupported: integration worktree and acceptance authority are not wired; use swarm.stage=shadow or fleet lanes")
		return core.PhaseResponse{Phase: d.Name(), Verdict: core.VerdictFAIL,
			Diagnostics: []core.Diagnostic{{Severity: "error", Message: err.Error()}}}, err
	}

	if st == stageAdvisory {
		resp, err := d.inner.Run(ctx, req)
		sr, derr := swarm.Dispatch(context.WithoutCancel(ctx), plan, swarm.DispatchRequest{
			ProjectRoot: req.ProjectRoot, Cycle: req.Cycle, Workspace: req.Workspace,
		}, d.dispatchDeps(req))
		annotate(&resp, plan, sr, derr, "advisory")
		return resp, err
	}

	sr, derr := swarm.Dispatch(ctx, plan, swarm.DispatchRequest{
		ProjectRoot: req.ProjectRoot, Cycle: req.Cycle, Workspace: req.Workspace,
	}, d.dispatchDeps(req))
	return d.enforce(ctx, req, plan, sr, derr)
}

func (d *Decorator) dispatchDeps(req core.PhaseRequest) swarm.Deps {
	manifestPath := filepath.Join(req.Workspace, ".swarm", "sessions.json")
	reg := swarm.NewSessionRegistry(manifestPath, req.Cycle, d.inner.Name(), os.Getpid())
	deps := swarm.Deps{
		Launcher:    bridgeLauncher{bridge: d.bridge, env: req.Env},
		Registry:    reg,
		Killer:      swarm.ExecSessionKiller{KillGroup: groupKiller, KillTmux: swarm.ExecTmuxKill},
		Concurrency: d.concurrency,
	}
	if d.mode == swarm.ModeWriter {
		deps.Provisioner = swarm.NewGitWorkerProvisioner(nil, d.cfg.WorktreeBase)
		deps.PortBase = d.cfg.PortBase
	}
	return deps
}

func (d *Decorator) enforce(ctx context.Context, req core.PhaseRequest, plan swarm.SwarmPlan, sr swarm.SwarmResult, derr error) (core.PhaseResponse, error) {
	resp := core.PhaseResponse{
		Phase:   d.inner.Name(),
		Verdict: core.VerdictPASS,
		CostUSD: sr.TotalCostUSD(),
		Tokens:  sr.TotalTokens(),
		Signals: map[string]any{},
	}
	annotate(&resp, plan, sr, derr, "enforce")

	if derr != nil || !sr.AllOK() {
		resp.Verdict = core.VerdictFAIL
		return resp, derr
	}

	if d.mode == swarm.ModeWriter {
		mr := swarm.RunMergeTrain(ctx, plan.IntegrationBranch, sr.MergeOrder, branchByID(plan),
			swarm.MergeTrainDeps{Merger: swarm.ExecGitMerger{IntegrationWorktree: sr.IntegrationWorktree}})
		resp.Signals["swarm.merged"] = mr.AllMerged
		if !mr.AllMerged {
			resp.Verdict = core.VerdictFAIL
			return resp, fmt.Errorf("swarm merge-train failed for %s", d.inner.Name())
		}
		resp.ArtifactsDir = sr.IntegrationWorktree
	}

	if d.mode == swarm.ModeReader {
		order := make([]string, len(sr.Workers))
		artifactByID := make(map[string]string, len(sr.Workers))
		for i, w := range sr.Workers {
			order[i] = w.WorkerID
			body, rerr := os.ReadFile(w.ArtifactPath)
			if rerr != nil {
				artifactByID[w.WorkerID] = fmt.Sprintf("[no artifact at %s: %v]", w.ArtifactPath, rerr)
				continue
			}
			artifactByID[w.WorkerID] = string(body)
		}
		synthPath := filepath.Join(req.Workspace, d.inner.Name()+"-report.md")
		if werr := os.WriteFile(synthPath, []byte(swarm.Synthesize(order, artifactByID)), 0o644); werr != nil {
			resp.Verdict = core.VerdictFAIL
			return resp, fmt.Errorf("swarm reader synthesis write %s: %w", synthPath, werr)
		}
		resp.ArtifactsDir = req.Workspace
		resp.Signals["swarm.synthesis"] = synthPath
	}
	return resp, nil
}

func annotate(resp *core.PhaseResponse, plan swarm.SwarmPlan, sr swarm.SwarmResult, derr error, stageName string) {
	if resp.Signals == nil {
		resp.Signals = map[string]any{}
	}
	resp.Signals["swarm.stage"] = stageName
	resp.Signals["swarm.mode"] = string(plan.Mode)
	resp.Signals["swarm.worker_count"] = len(sr.Workers)
	resp.Signals["swarm.all_ok"] = sr.AllOK()
	if derr != nil {
		resp.Signals["swarm.error"] = derr.Error()
	}
}

func branchByID(plan swarm.SwarmPlan) map[string]string {
	out := make(map[string]string, len(plan.Workers))
	for _, w := range plan.Workers {
		out[w.WorkerID] = w.Branch
	}
	return out
}

func parseSwarmStage(s string) stage {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "advisory":
		return stageAdvisory
	case "enforce":
		return stageEnforce
	default:
		return stageOff
	}
}

func loadPlan(workspace string) (swarm.SwarmPlan, bool) {
	for _, name := range []string{"swarm-plan.json", "swarm-plan.md"} {
		data, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			continue
		}
		if plan, perr := swarm.ParsePlan(string(data)); perr == nil {
			return plan, true
		}
	}
	return swarm.SwarmPlan{}, false
}

type bridgeLauncher struct {
	bridge core.Bridge
	env    map[string]string
}

func (l bridgeLauncher) Launch(ctx context.Context, r swarm.LaunchRequest) (swarm.LaunchResult, error) {
	overlaySkills := policy.ResolveLaunchOverlaysFailOpen(r.ProjectRoot, r.Agent, r.CLI, r.Model)
	resp, err := core.LaunchWithFreshSession(ctx, l.bridge, core.BridgeRequest{
		CLI: r.CLI, Model: r.Model, Profile: r.Profile, Agent: r.Agent,
		SessionName: r.SessionName, Prompt: r.Prompt,
		Workspace: r.Workspace, Worktree: r.Worktree, ProjectRoot: r.ProjectRoot,
		ArtifactPath: r.ArtifactPath, Cycle: r.Cycle, Env: mergeEnv(l.env, r.Env),
		Skills: overlaySkills,
	})
	if err != nil {
		return swarm.LaunchResult{}, err
	}
	return swarm.LaunchResult{ExitCode: resp.ExitCode, CostUSD: resp.CostUSD, Tokens: resp.Tokens}, nil
}

func mergeEnv(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func (d *Decorator) PersonaAvailable() error {
	if p, ok := d.inner.(core.PersonaProber); ok {
		return p.PersonaAvailable()
	}
	return nil
}

func (d *Decorator) SignalsWired() bool {
	if w, ok := d.inner.(interface{ SignalsWired() bool }); ok {
		return w.SignalsWired()
	}
	return false
}

func (d *Decorator) ContractVerifierWired() bool {
	if w, ok := d.inner.(interface{ ContractVerifierWired() bool }); ok {
		return w.ContractVerifierWired()
	}
	return false
}

func (d *Decorator) HostEffectsWired() bool {
	if w, ok := d.inner.(interface{ HostEffectsWired() bool }); ok {
		return w.HostEffectsWired()
	}
	return false
}
