package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// OrphanVerdict is the per-branch outcome of a PruneSupersededOrphans walk.
type OrphanVerdict struct {
	// Ref is the branch's short name (e.g. "cycle-100").
	Ref string
	// Superseded is true when the branch is already landed on base (ancestor
	// or patch-id functional duplicate).
	Superseded bool
	// Pruned is true only when the branch was both Superseded and safe to
	// delete (hasOpenPR reported false), and the delete succeeded.
	Pruned bool
	// KeptCheckedOut, KeptBound and KeptDeleteFailed name why a superseded
	// branch was kept; at most one is set. With none set, a kept superseded
	// branch was kept behind an open PR.
	KeptCheckedOut   bool
	KeptBound        bool
	KeptDeleteFailed bool
}

// PruneSupersededOrphans walks the local `cycle-*` branches in dir and, for
// each, decides whether it is a superseded functional duplicate of base and
// whether it may be pruned. A superseded branch checked out in any worktree
// (KeptCheckedOut) or named by a continuation-registry binding under dir
// (KeptBound) is kept before any PR check or delete. Otherwise it is deleted
// (`git branch -D`) ONLY when hasOpenPR(ref) reports false — an open PR /
// remote leaves it flagged-but-kept (verify_remote_pr_before_branch_delete).
// A delete git refuses is reported on that ref (KeptDeleteFailed) and the walk
// continues. Non-superseded (different-goal) branches are reported untouched.
// An error from listing refs or worktrees, reading the registry, running git,
// or from hasOpenPR aborts the walk before any further delete.
func PruneSupersededOrphans(ctx context.Context, dir, base string, hasOpenPR func(ref string) (bool, error)) ([]OrphanVerdict, error) {
	out, code, err := gitCapture(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/cycle-*")
	if err != nil {
		return nil, fmt.Errorf("prune orphans: list cycle-* refs: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("prune orphans: for-each-ref exited %d", code)
	}
	checkedOut, err := checkedOutBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	bound, err := continuationBoundBranches(dir)
	if err != nil {
		return nil, err
	}

	var verdicts []OrphanVerdict
	for _, ref := range strings.Split(out, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		superseded, err := refSuperseded(ctx, dir, ref, base)
		if err != nil {
			return nil, err
		}
		v := OrphanVerdict{Ref: ref, Superseded: superseded}
		if superseded {
			if v, err = keepOrPrune(ctx, dir, v, checkedOut[ref], bound[ref], hasOpenPR); err != nil {
				return nil, err
			}
		}
		verdicts = append(verdicts, v)
	}
	return verdicts, nil
}

func keepOrPrune(ctx context.Context, dir string, v OrphanVerdict, isCheckedOut, isBound bool, hasOpenPR func(ref string) (bool, error)) (OrphanVerdict, error) {
	switch {
	case isCheckedOut:
		v.KeptCheckedOut = true
		return v, nil
	case isBound:
		v.KeptBound = true
		return v, nil
	}
	open, err := hasOpenPR(v.Ref)
	if err != nil {
		return v, fmt.Errorf("prune orphans: hasOpenPR(%s): %w", v.Ref, err)
	}
	if open {
		return v, nil
	}
	_, code, err := gitCapture(ctx, dir, "branch", "-D", v.Ref)
	if err != nil {
		return v, fmt.Errorf("prune orphans: delete %s: %w", v.Ref, err)
	}
	v.Pruned = code == 0
	v.KeptDeleteFailed = code != 0
	return v, nil
}

func checkedOutBranches(ctx context.Context, dir string) (map[string]bool, error) {
	out, code, err := gitCapture(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("prune orphans: list worktrees: %w", err)
	}
	if code != 0 {
		return nil, fmt.Errorf("prune orphans: worktree list exited %d", code)
	}
	branches := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if b, ok := strings.CutPrefix(strings.TrimSpace(line), "branch refs/heads/"); ok {
			branches[b] = true
		}
	}
	return branches, nil
}

func continuationBoundBranches(dir string) (map[string]bool, error) {
	entries, err := continuation.ListRegistryEntries(dir)
	if err != nil {
		return nil, fmt.Errorf("prune orphans: %w", err)
	}
	branches := map[string]bool{}
	for _, c := range entries {
		if c.Branch != "" {
			branches[c.Branch] = true
		}
	}
	return branches, nil
}
