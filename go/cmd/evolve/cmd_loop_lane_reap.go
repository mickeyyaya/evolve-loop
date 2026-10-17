package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const laneReapTimeout = 30 * time.Second

type dispatchReap func(ctx context.Context, ids []string) proctree.Report

func (b *loopBatchCoordinator) laneReap() func(orchestrator.Lane) {
	root := b.cfg.ProjectRoot
	return func(lane orchestrator.Lane) {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(b.ctx), laneReapTimeout)
		defer cancel()
		run := newGCRun(ctx, false, b.stderr, b.stderr)
		reapLane(ctx, root, lane, gcProcesses().list, func(_ context.Context, ids []string) proctree.Report {
			return run.reapDispatches(root, ids)
		}, b.stderr)
	}
}

func reapLane(ctx context.Context, projectRoot string, lane orchestrator.Lane, list proctree.Lister, reap dispatchReap, warn io.Writer) {
	cycle, ok := orchestrator.LaneCycle(filepath.Dir(paths.RunWorkspace(projectRoot, 0)), lane)
	if !ok {
		fmt.Fprintf(warn, "[loop] WARN: orchestrator: no cycle holds the stopped lane %s; `evolve gc` reaps its dispatches\n", lane.Key)
		return
	}
	table, err := list(ctx)
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: orchestrator: the dispatch reap of the stopped lane %s (cycle %d) did not list the processes: %v; `evolve gc` reaps them\n", lane.Key, cycle, err)
		return
	}
	ids := laneDispatchIDs(table, projectRoot, cycle)
	if len(ids) == 0 {
		return
	}
	rep := reap(ctx, ids)
	fmt.Fprintf(warn, "[loop] orchestrator: the dispatch reap of the stopped lane %s (cycle %d) stopped %d process(es) of %d dispatch(es) (SIGKILL: %d, errors: %d, survivors: %d)\n",
		lane.Key, cycle, len(rep.Terminated), len(ids), len(rep.Killed), len(rep.Errors), len(rep.Survivors))
	for _, e := range rep.Errors {
		fmt.Fprintf(warn, "[loop] WARN: orchestrator: the dispatch reap of the stopped lane %s: %s\n", lane.Key, e)
	}
}

func laneDispatchIDs(table []proctree.Process, projectRoot string, cycle int) []string {
	root := filepath.Clean(projectRoot)
	var ids []string
	for _, p := range table {
		tag := p.Env[ipcenv.DispatchIDKey]
		id, ok := proctree.ParseDispatchID(tag)
		if ok && id.Cycle == cycle && filepath.Clean(p.Env[gc.ProjectRootEnvKey]) == root && !slices.Contains(ids, tag) {
			ids = append(ids, tag)
		}
	}
	slices.Sort(ids)
	return ids
}
