package main

import (
	"fmt"
	"os"
	"slices"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

const gcDispatchGrace = 2 * time.Second

type gcProcessHost struct {
	list   proctree.Lister
	signal proctree.Signaler
	sleep  func(time.Duration)
	alive  func(int) bool
}

var gcInjectedProcessHost *gcProcessHost

func gcProcesses() gcProcessHost {
	if gcInjectedProcessHost != nil {
		return *gcInjectedProcessHost
	}
	return gcProcessHost{list: proctree.ExecLister(), signal: syscall.Kill, sleep: time.Sleep, alive: swarm.ExecPidAlive}
}

func (r gcRun) dispatchOptions(host gcProcessHost, projectRoot string, tailTTL time.Duration) gc.DispatchProcessOptions {
	return gc.DispatchProcessOptions{ProjectRoot: projectRoot, PidAlive: host.alive, Now: time.Now(),
		CycleClosed:  func(n int) bool { return dossier.ClosedOut(projectRoot, n) },
		RecordedTree: r.recordedTree(proctree.TreeDir(projectRoot)),
		TailTTL:      tailTTL}
}

func (h gcProcessHost) reaper() proctree.Reaper {
	return proctree.Reaper{List: h.list, Signal: h.signal, Sleep: h.sleep, Grace: gcDispatchGrace, Self: os.Getpid()}
}

func (r gcRun) reapDispatches(projectRoot string, ids []string) proctree.Report {
	host := gcProcesses()
	stale := gc.StaleDispatch(r.dispatchOptions(host, projectRoot, 0))
	return host.reaper().Reap(r.ctx, func(p proctree.Process) bool {
		return slices.Contains(ids, p.Env[ipcenv.DispatchIDKey]) && stale(p)
	})
}

func (r gcRun) dispatchProcesses(projectRoot string, tempTTLHours int) bool {
	host := gcProcesses()
	o := r.dispatchOptions(host, projectRoot, time.Duration(tempTTLHours)*time.Hour)
	table, err := host.list(r.ctx)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: dispatch process sweep skipped: %v\n", err)
		return true
	}
	items := gc.PlanDispatchProcesses(table, o)
	r.summary("%d dispatch process(es) would be stopped", "stopping %d dispatch process(es)", len(items))
	verb := "STOP"
	if r.dryRun {
		verb = "WOULD-STOP"
	}
	for _, it := range items {
		fmt.Fprintf(r.stdout, "  %s pid=%d rule=%s comm=%q %s\n", verb, it.Process.Pid, it.Rule, it.Process.Comm, gcProcessDetail(it))
	}
	if r.dryRun || len(items) == 0 {
		return false
	}
	return r.reportDispatchReap(host.reaper().Reap(r.ctx, proctree.AnyOf(gc.StaleDispatch(o), gc.OrphanLogTail(o))))
}

func gcProcessDetail(it gc.DispatchProcessItem) string {
	if it.Rule == gc.RuleStaleDispatch {
		return fmt.Sprintf("dispatch=%q", it.Process.Env[ipcenv.DispatchIDKey])
	}
	return fmt.Sprintf("args=%q", it.Process.Args)
}

func (r gcRun) recordedTree(dir string) func(string) []proctree.Identity {
	return func(id string) []proctree.Identity {
		tree, err := proctree.LoadTree(dir, id)
		if err != nil {
			fmt.Fprintf(r.stderr, "evolve gc: WARN: dispatch %q: %v; only the tag proof applies\n", id, err)
		}
		return tree
	}
}

func (r gcRun) reportDispatchReap(rep proctree.Report) bool {
	fmt.Fprintf(r.stdout, "evolve gc: stopped %d dispatch process(es) (SIGKILL: %d)\n", len(rep.Terminated), len(rep.Killed))
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: dispatch process error: %s\n", e)
	}
	for _, p := range rep.Survivors {
		fmt.Fprintf(r.stderr, "evolve gc: ERROR: pid %d (%s) is alive after SIGKILL\n", p.Pid, p.Comm)
	}
	return len(rep.Errors) > 0 || len(rep.Survivors) > 0
}
