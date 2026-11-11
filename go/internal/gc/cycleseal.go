package gc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type SealedCycleReport struct {
	Held   string         `json:"held,omitempty"`
	Reaped []Process      `json:"reaped,omitempty"`
	Items  []WorktreeItem `json:"items,omitempty"`
	Errors []string       `json:"errors,omitempty"`
}

func (r *SealedCycleReport) note(err error) bool {
	if err == nil {
		return false
	}
	r.Errors = append(r.Errors, err.Error())
	return true
}

func (r SealedCycleReport) Summary() string {
	if r.Held != "" {
		return "held: " + r.Held
	}
	s := fmt.Sprintf("stopped %d process(es), released %d item(s)", len(r.Reaped), len(r.Items))
	if len(r.Errors) > 0 {
		s += fmt.Sprintf(", %d error(s): %s", len(r.Errors), strings.Join(r.Errors, "; "))
	}
	return s
}

func SweepSealedCycle(ctx context.Context, o WorktreeOptions, cycle int, kill func(pid int) error) SealedCycleReport {
	var rep SealedCycleReport
	hold, err := o.sealHold(cycle)
	if rep.note(err) || hold != "" {
		rep.Held = hold
		return rep
	}
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if rep.note(err) {
		return rep
	}
	defer release()
	procs, err := o.listProcesses(ctx)
	rep.note(err)
	reaped := reapProcesses(o.cycleOrphans(procs, cycle), kill)
	rep.Reaped = reaped.Reaped
	rep.Errors = append(rep.Errors, reaped.Errors...)
	o.releaseSealedLanes(cycle, &rep)
	return rep
}

func (o WorktreeOptions) sealHold(cycle int) (string, error) {
	held, err := o.laneHolds()
	if err != nil {
		return "", err
	}
	switch {
	case held.pausedCycles[cycle] || o.planePaused(cycle):
		return "the cycle is paused with a resumable checkpoint", nil
	case held.boundTo(cycle):
		return "the cycle is bound to a continuation", nil
	case o.runLiveElsewhere(cycle):
		return "the run of the cycle holds a live lease", nil
	}
	return "", nil
}

func (o WorktreeOptions) runLiveElsewhere(cycle int) bool {
	alive := o.PidAlive
	o.PidAlive = func(pid int) bool { return pid != os.Getpid() && (alive == nil || alive(pid)) }
	return o.runLive(cycle)
}

func (o WorktreeOptions) planePaused(cycle int) bool {
	for _, state := range []string{
		filepath.Join(o.EvolveDir, core.CycleStateFile),
		filepath.Join(o.EvolveDir, "runs", fmt.Sprintf("cycle-%d", cycle), core.CycleStateFile),
	} {
		rp, err := core.CheckpointResumable(state, o.ProjectRoot, core.ResumeOptions{})
		if err == nil && rp.CycleID == cycle && core.IsResumableReason(rp.Reason) {
			return true
		}
	}
	return false
}

func (h laneHolds) boundTo(cycle int) bool {
	for branch := range h.boundBranches {
		if n, ok := LeafCycleNumber(branch); ok && n == cycle {
			return true
		}
	}
	return false
}

func cycleLeaf(leaf string, cycle int) bool {
	n, ok := LeafCycleNumber(leaf)
	return ok && n == cycle && strings.HasPrefix(leaf, "cycle-")
}

func (o WorktreeOptions) cycleOrphans(procs []Process, cycle int) []Process {
	var out []Process
	for _, p := range procs {
		if leaf, ok := leafUnder(o.WorktreeBase, p.Cwd); ok && p.Ppid == 1 && cycleLeaf(leaf, cycle) {
			out = append(out, p)
		}
	}
	return out
}

type sealedLane struct {
	path   string
	branch string
}

func (o WorktreeOptions) sealedLanes(cycle int) ([]sealedLane, *mergedSet, error) {
	porcelain, err := o.git(o.ProjectRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, nil, err
	}
	merged, err := o.mergedBranches()
	if err != nil {
		return nil, nil, err
	}
	listed, err := o.git(o.ProjectRoot, "branch", "--list", "cycle-*")
	if err != nil {
		return nil, nil, err
	}
	var lanes []sealedLane
	seen := map[string]bool{}
	for _, e := range parseWorktreePorcelain(porcelain) {
		leaf := filepath.Base(e.path)
		if e.branch != "" && underBase(o.WorktreeBase, e.path) && cycleLeaf(leaf, cycle) {
			lanes = append(lanes, sealedLane{path: filepath.Join(o.WorktreeBase, leaf), branch: e.branch})
			seen[e.branch] = true
		}
	}
	for _, b := range parseBranchList(listed) {
		if cycleLeaf(b, cycle) && !seen[b] {
			lanes = append(lanes, sealedLane{branch: b})
		}
	}
	return lanes, merged, nil
}

func (o WorktreeOptions) releaseSealedLanes(cycle int, rep *SealedCycleReport) {
	lanes, merged, err := o.sealedLanes(cycle)
	if rep.note(err) {
		return
	}
	for _, l := range lanes {
		items, err := o.releaseSealedLane(l, merged)
		rep.note(err)
		rep.Items = append(rep.Items, items...)
	}
}

const sealedReason = "the cycle sealed"

func (o WorktreeOptions) releaseSealedLane(l sealedLane, merged *mergedSet) ([]WorktreeItem, error) {
	land, err := merged.landing(l.branch)
	if err != nil {
		return nil, err
	}
	del := WorktreeItem{Path: l.path, Branch: l.branch, Head: o.branchHead(l.branch), Action: WorktreeActionDeleteBranch, Reason: sealedReason, Bundle: land == landingNone}
	if del.Bundle {
		if err := o.bundleBranch(del); err != nil {
			return nil, fmt.Errorf("gc: keep %s: bundle failed, lane and branch kept: %w", l.branch, err)
		}
	}
	var items []WorktreeItem
	if l.path != "" {
		item, err := o.releaseLaneTree(l)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := o.dropSealedBranch(del); err != nil {
		return items, err
	}
	return append(items, del), nil
}

func (o WorktreeOptions) releaseLaneTree(l sealedLane) (WorktreeItem, error) {
	item := WorktreeItem{Path: l.path, Branch: l.branch, Action: WorktreeActionRemove, Reason: sealedReason}
	dirty, err := o.isDirty(l.path)
	if err != nil {
		return item, err
	}
	if dirty {
		if err := o.salvageWorktree(l.path, l.branch); err != nil {
			return item, fmt.Errorf("gc: keep %s: salvage failed, tree kept: %w", l.path, err)
		}
		item.Action = WorktreeActionSalvageRemove
	}
	_, err = o.git(o.ProjectRoot, "worktree", "remove", "--force", l.path)
	return item, err
}

func (o WorktreeOptions) dropSealedBranch(it WorktreeItem) error {
	if !it.Bundle {
		return o.deleteBranch(it)
	}
	if err := o.refuseCheckedOut(it.Branch); err != nil {
		return err
	}
	return o.dropRef(it)
}
