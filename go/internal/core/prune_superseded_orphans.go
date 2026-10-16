package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

type OrphanVerdict struct {
	Ref              string
	Superseded       bool
	Pruned           bool
	KeptCheckedOut   bool
	KeptBound        bool
	KeptDeleteFailed bool
}

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
