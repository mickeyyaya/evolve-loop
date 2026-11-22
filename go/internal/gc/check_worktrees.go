package gc

import (
	"context"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type worktreeCheck struct{}

func (worktreeCheck) Name() string { return "worktrees" }

func (worktreeCheck) options(cc CheckContext) WorktreeOptions {
	opts := cc.Worktrees
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = cc.ProjectRoot
	}
	if opts.EvolveDir == "" {
		if cc.EvolveDir != "" {
			opts.EvolveDir = cc.EvolveDir
		} else if opts.ProjectRoot != "" {
			opts.EvolveDir = filepath.Join(opts.ProjectRoot, ".evolve")
		}
	}
	if opts.WorktreeBase == "" && opts.ProjectRoot != "" {
		opts.WorktreeBase = filepath.Join(opts.ProjectRoot, ".evolve", "worktrees")
	}
	if opts.Policy == (WorktreesPolicy{}) {
		opts.Policy = cc.Policy.Worktrees
	}
	if opts.Exec == nil {
		opts.Exec = sysexec.DefaultRunner
	}
	if !opts.Landing.isSet() {
		opts.Landing = LandingTarget{Remote: "origin", Branch: "main"}
	}
	return opts
}

func (w worktreeCheck) Scan(_ context.Context, cc CheckContext) ([]CheckItem, error) {
	m, err := PlanWorktrees(w.options(cc))
	if err != nil {
		return nil, err
	}
	items := make([]CheckItem, 0, len(m.Items))
	for _, it := range m.Items {
		items = append(items, CheckItem{
			Subject:  worktreeCheckSubject(it),
			Action:   string(it.Action),
			Rule:     worktreeCheckRule(it.Action),
			Evidence: worktreeCheckEvidence(it),
		})
	}
	return items, nil
}

func (w worktreeCheck) Apply(_ context.Context, cc CheckContext) error {
	opts := w.options(cc)
	m, err := PlanWorktrees(opts)
	if err != nil {
		return err
	}
	return ApplyWorktrees(opts, m)
}

func worktreeCheckSubject(it WorktreeItem) string {
	if it.Action == WorktreeActionDeleteBranch && it.Branch != "" {
		return it.Branch
	}
	if it.Path != "" {
		return it.Path
	}
	return it.Branch
}

func worktreeCheckRule(action WorktreeAction) string {
	switch action {
	case WorktreeActionRemove:
		return "worktree.landed_clean_dead"
	case WorktreeActionDeleteBranch:
		return "worktree.branch_landed"
	case WorktreeActionFlagDirty:
		return "worktree.dirty_preserve"
	case WorktreeActionFlagUnmerged:
		return "worktree.unmerged_preserve"
	case WorktreeActionSalvageRemove:
		return "worktree.salvage_expired"
	default:
		return "worktree." + string(action)
	}
}

func worktreeCheckEvidence(it WorktreeItem) string {
	if it.Reason != "" {
		return it.Reason
	}
	return string(it.Action)
}
