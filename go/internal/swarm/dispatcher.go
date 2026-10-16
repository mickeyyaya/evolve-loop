package swarm

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

const DefaultPortBase = 51000

func portBaseOrDefault(base int) int {
	if base <= 0 {
		return DefaultPortBase
	}
	return base
}

type DispatchRequest struct {
	ProjectRoot string
	Cycle       int
	Workspace   string
}

func Dispatch(ctx context.Context, plan SwarmPlan, req DispatchRequest, deps Deps) (SwarmResult, error) {
	res := SwarmResult{Mode: plan.Mode, IntegrationBranch: plan.IntegrationBranch}

	worktrees := make(map[string]string, len(plan.Workers))
	if plan.Mode == ModeWriter && deps.Provisioner != nil {
		integWT, err := deps.Provisioner.CreateIntegration(ctx, req.ProjectRoot, req.Cycle)
		if err != nil {
			return res, fmt.Errorf("provision integration branch: %w", err)
		}
		res.IntegrationWorktree = integWT
		for _, w := range plan.Workers {
			wt, err := deps.Provisioner.CreateWorker(ctx, req.ProjectRoot, req.Cycle, w.WorkerID, plan.IntegrationBranch)
			if err != nil {
				for _, done := range worktrees {
					_ = deps.Provisioner.Cleanup(ctx, req.ProjectRoot, done)
				}
				return res, fmt.Errorf("provision worker %s: %w", w.WorkerID, err)
			}
			worktrees[w.WorkerID] = wt
		}
	}

	rootCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	conc := deps.Concurrency
	if conc <= 0 {
		conc = len(plan.Workers)
	}
	sem := make(chan struct{}, conc)
	results := make([]WorkerResult, len(plan.Workers))
	var wg sync.WaitGroup
	var fatalOnce sync.Once
	var fatalErr error

	for i, w := range plan.Workers {
		wg.Add(1)
		port := 0
		if plan.Mode == ModeWriter {
			port = portBaseOrDefault(deps.PortBase) + i
		}
		go func(i int, w WorkerSpec, port int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-rootCtx.Done():
				results[i] = WorkerResult{WorkerID: w.WorkerID, Agent: w.agentName(plan), Err: rootCtx.Err()}
				return
			}
			wr := launchWorker(rootCtx, plan, req, w, worktrees[w.WorkerID], port, deps)
			results[i] = wr
			if wr.Err != nil {
				fatalOnce.Do(func() { fatalErr = wr.Err; cancel() })
			}
		}(i, w, port)
	}
	wg.Wait()

	if deps.Registry != nil && deps.Killer != nil {
		_ = Reap(context.Background(), deps.Registry, deps.Killer)
	}

	res.Workers = results
	if plan.Mode == ModeWriter {
		if order, err := TopoOrder(plan.Workers); err == nil {
			res.MergeOrder = order
		}
	}
	return res, fatalErr
}

func launchWorker(ctx context.Context, plan SwarmPlan, req DispatchRequest, w WorkerSpec, worktree string, port int, deps Deps) WorkerResult {
	agent := w.agentName(plan)
	wr := WorkerResult{WorkerID: w.WorkerID, Agent: agent, Branch: w.Branch, Worktree: worktree}

	var sessionName, tmuxSession string
	if bridge.IsTmuxDriver(w.CLI) {
		sessionName = fmt.Sprintf("swarm-c%d-%s", req.Cycle, w.WorkerID)
		tmuxSession = bridge.NamedSessionName(sessionName)
	}

	if deps.Registry != nil {
		if err := deps.Registry.Register(SessionHandle{
			WorkerID: w.WorkerID, Agent: agent, TmuxSession: tmuxSession,
			Worktree: worktree, Branch: w.Branch,
		}); err != nil {
			wr.Err = fmt.Errorf("pre-register session %s: %w", w.WorkerID, err)
			return wr
		}
	}

	workspace := filepath.Join(req.Workspace, agent)
	wr.ArtifactPath = filepath.Join(workspace, agent+"-report.md")

	var env map[string]string
	if port > 0 {
		env = map[string]string{"PORT": strconv.Itoa(port)}
	}
	lr, err := deps.Launcher.Launch(ctx, LaunchRequest{
		CLI:          w.CLI,
		Model:        w.Model,
		Profile:      w.Profile,
		Agent:        agent,
		SessionName:  sessionName,
		Prompt:       workerPrompt(w),
		Workspace:    workspace,
		Worktree:     worktree,
		ProjectRoot:  req.ProjectRoot,
		ArtifactPath: wr.ArtifactPath,
		Cycle:        req.Cycle,
		Env:          env,
	})
	if err != nil {
		wr.Err = err
		return wr
	}
	wr.ExitCode = lr.ExitCode
	wr.CostUSD = lr.CostUSD
	wr.Tokens = lr.Tokens
	return wr
}

func (w WorkerSpec) agentName(plan SwarmPlan) string {
	prefix := string(plan.Mode)
	if plan.TaskID != "" {
		prefix = plan.TaskID
	}
	return prefix + "-" + w.WorkerID
}

func workerPrompt(w WorkerSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Worker %s\n\n## Scope\n%s\n", w.WorkerID, w.Scope)
	if len(w.TargetFiles) > 0 {
		fmt.Fprintf(&b, "\n## Files you own (do not touch others)\n%s\n", strings.Join(w.TargetFiles, "\n"))
	}
	if len(w.Acceptance) > 0 {
		fmt.Fprintf(&b, "\n## Acceptance\n%s\n", strings.Join(w.Acceptance, "\n"))
	}
	return b.String()
}
