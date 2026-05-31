package swarm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// DispatchRequest carries the cycle-scoped context the dispatcher needs that is
// not part of the plan itself (the plan describes WHAT to run; this describes
// WHERE). Supplied by the orchestrator seam (v4) or a test.
type DispatchRequest struct {
	ProjectRoot string
	Cycle       int
	Workspace   string // base workspace; each worker gets a <workspace>/<agent> subdir
}

// Dispatch fans out a validated SwarmPlan to N parallel workers, returning their
// collected results. It is the structured-concurrency spine of the harness:
//
//   - bounded concurrency via a buffered-channel semaphore (Deps.Concurrency),
//     mirroring internal/fanoutdispatch (raw sync — no errgroup dependency);
//   - the call BLOCKS on wg.Wait() and reaps every registered session before
//     returning, so no worker session can outlive the dispatch scope (the live
//     half of the teardown guarantee);
//   - the first fatal launch error cancels the derived context (cancel-on-fatal)
//     so siblings stop promptly.
//
// Dispatch does NOT merge (writers) or synthesize (readers) — that is the v4
// fan-in step. It provisions per-worker isolation, launches, records sessions,
// and returns WorkerResults for the caller to reduce. The plan MUST already be
// validated (Validate(plan).OK); Dispatch trusts the partition.
//
// PROVISIONING NOTE: writer worktree creation touches shared .git/worktrees
// metadata, so provisioning is serialized up-front (before the parallel launch
// section) — only the launches run concurrently.
//
// V4 WIRING REQUIREMENT (orphan-on-cancel): launchWorker registers a session
// only after Launch returns success. A real Launcher that spawns a tmux session
// and is THEN cancelled could leave a live session this code never registered,
// which the post-wg Reap would miss. The production Launcher adapter MUST either
// register the session at spawn, or return the session identity even on error so
// we can register-then-reap. The crash-safe `evolve swarm reap` is the backstop.
func Dispatch(ctx context.Context, plan SwarmPlan, req DispatchRequest, deps Deps) (SwarmResult, error) {
	res := SwarmResult{Mode: plan.Mode, IntegrationBranch: plan.IntegrationBranch}

	// Phase 1 (serialized): provision integration + per-worker worktrees.
	// Readers need no worktrees (Provisioner nil) — they share the read-only tree.
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
				// Clean up the worktrees already provisioned this call so a
				// mid-provisioning failure doesn't leak them (the launch section
				// never starts, so the post-wg Reap won't cover these).
				for _, done := range worktrees {
					_ = deps.Provisioner.Cleanup(ctx, req.ProjectRoot, done)
				}
				return res, fmt.Errorf("provision worker %s: %w", w.WorkerID, err)
			}
			worktrees[w.WorkerID] = wt
		}
	}

	// Phase 2 (parallel): launch workers under a bounded semaphore.
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
		go func(i int, w WorkerSpec) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-rootCtx.Done():
				results[i] = WorkerResult{WorkerID: w.WorkerID, Agent: w.agentName(plan), Err: rootCtx.Err()}
				return
			}
			wr := launchWorker(rootCtx, plan, req, w, worktrees[w.WorkerID], deps)
			results[i] = wr // each index written by exactly one goroutine — race-free
			if wr.Err != nil {
				fatalOnce.Do(func() { fatalErr = wr.Err; cancel() })
			}
		}(i, w)
	}
	wg.Wait()

	// Teardown (live half): reap every registered session before returning.
	// context.Background so teardown still runs even if rootCtx was cancelled.
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

// launchWorker realizes one WorkerSpec → LaunchRequest, launches, and registers
// the session.
func launchWorker(ctx context.Context, plan SwarmPlan, req DispatchRequest, w WorkerSpec, worktree string, deps Deps) WorkerResult {
	agent := w.agentName(plan)
	wr := WorkerResult{WorkerID: w.WorkerID, Agent: agent, Branch: w.Branch, Worktree: worktree}

	workspace := filepath.Join(req.Workspace, agent)
	lr, err := deps.Launcher.Launch(ctx, LaunchRequest{
		CLI:          w.CLI,
		Model:        w.Model,
		Profile:      w.Profile,
		Agent:        agent,
		Prompt:       workerPrompt(w),
		Workspace:    workspace,
		Worktree:     worktree,
		ProjectRoot:  req.ProjectRoot,
		ArtifactPath: filepath.Join(workspace, agent+"-report.md"),
		Cycle:        req.Cycle,
	})
	if err != nil {
		wr.Err = err
		return wr
	}
	wr.ExitCode = lr.ExitCode
	wr.CostUSD = lr.CostUSD

	if deps.Registry != nil {
		_ = deps.Registry.Register(SessionHandle{
			WorkerID: w.WorkerID, Agent: agent, TmuxSession: lr.TmuxSession,
			PGID: lr.PGID, Worktree: worktree, Branch: w.Branch,
		})
	}
	return wr
}

// agentName is the collision-safe tmux/inbox key: "<task-or-mode>-w<i>". The
// worker_id (unique post-validation) is the suffix.
func (w WorkerSpec) agentName(plan SwarmPlan) string {
	prefix := string(plan.Mode)
	if plan.TaskID != "" {
		prefix = plan.TaskID
	}
	return prefix + "-" + w.WorkerID
}

// workerPrompt builds a minimal per-worker prompt from its scope + acceptance +
// owned files. The full persona-driven prompt is layered by the runner at the
// composition root (v4); this is the dispatch-level fallback.
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
