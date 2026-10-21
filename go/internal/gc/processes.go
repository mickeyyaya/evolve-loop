package gc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Process = proctree.Cwd

type ProcessReapReport struct {
	Reaped []Process `json:"reaped"`
	Errors []string  `json:"errors,omitempty"`
}

func ReapFinishedCycleOrphans(ctx context.Context, o WorktreeOptions, kill func(pid int) error) ProcessReapReport {
	var rep ProcessReapReport
	procs, err := o.listProcesses(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	for _, p := range o.FinishedCycleOrphans(procs) {
		if err := kill(p.Pid); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("signal pid %d (cwd %s): %v", p.Pid, p.Cwd, err))
			continue
		}
		rep.Reaped = append(rep.Reaped, p)
	}
	return rep
}

func (o WorktreeOptions) FinishedCycleOrphans(procs []Process) []Process {
	var out []Process
	for _, p := range procs {
		if p.Ppid == 1 && o.finishedCycleTree(p.Cwd) {
			out = append(out, p)
		}
	}
	return out
}

func (o WorktreeOptions) finishedCycleTree(cwd string) bool {
	leaf, ok := leafUnder(o.WorktreeBase, cwd)
	if !ok {
		return false
	}
	n, ok := LeafCycleNumber(leaf)
	tree := filepath.Join(o.WorktreeBase, leaf)
	if !strings.HasPrefix(leaf, "cycle-") || !ok || o.isLive(tree) {
		return false
	}
	if _, err := os.Stat(tree); os.IsNotExist(err) {
		return true
	}
	return dossier.ClosedOut(o.ProjectRoot, n)
}

func leafUnder(base, p string) (string, bool) {
	for _, b := range []string{filepath.Clean(base), resolvePath(base)} {
		rel, err := filepath.Rel(b, p)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return strings.SplitN(rel, string(filepath.Separator), 2)[0], true
	}
	return "", false
}

func (o WorktreeOptions) listProcesses(ctx context.Context) ([]Process, error) {
	return listCwdProcesses(ctx, o.Exec, o.ProjectRoot)
}

func listCwdProcesses(ctx context.Context, run sysexec.RunFunc, dir string) ([]Process, error) {
	procs, err := proctree.ListCwds(ctx, run, dir)
	if err != nil {
		return nil, fmt.Errorf("gc: %w", err)
	}
	return procs, nil
}
