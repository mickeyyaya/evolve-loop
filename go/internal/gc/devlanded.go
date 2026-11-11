package gc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const landedInMain = "is in main"

func (s *devSweep) landedEarly(ctx context.Context, it *DevItem) (bool, error) {
	tree := gitexec.Git{Dir: it.Path, Exec: s.o.Exec}
	if it.Branch == "" {
		return s.detachedLandedEarly(ctx, tree, it)
	}
	own, err := hasOwnWork(ctx, tree, it.Head)
	if err != nil || !own {
		return false, err
	}
	dirty, err := tree.DirtyPaths(ctx)
	if err != nil || len(dirty) > 0 {
		return false, err
	}
	landed, err := s.landed(ctx, *it)
	if err != nil || landed != landedInMain {
		return false, err
	}
	prs, err := s.openPRs(ctx)
	if err != nil || s.prsSkipped {
		return false, err
	}
	return len(prs[it.Branch]) == 0, nil
}

func (s *devSweep) detachedLandedEarly(ctx context.Context, tree gitexec.Git, it *DevItem) (bool, error) {
	dirty, err := tree.DirtyPaths(ctx)
	return s.detached[it.Path].merged && len(dirty) == 0, err
}

func hasOwnWork(ctx context.Context, tree gitexec.Git, head string) (bool, error) {
	out, err := tree.Output(ctx, "reflog", "show", "--format=%H", "HEAD")
	if err != nil {
		return false, fmt.Errorf("git reflog show HEAD: %w", err)
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return false, nil
	}
	return lines[len(lines)-1] != head, nil
}

func (s *devSweep) mergedTips(ctx context.Context) map[string][]string {
	if s.mergedRead || s.o.MergedPRs == nil {
		return s.merged
	}
	s.mergedRead = true
	tips, err := s.o.MergedPRs(ctx, s.o.ProjectRoot)
	switch {
	case errors.Is(err, ErrPRLookupUnavailable):
		s.notes = append(s.notes, "gh is unavailable: the merged pull request source was skipped, so landed rests on git evidence")
	case err != nil:
		s.notes = append(s.notes, "the merged pull request source failed, so landed rests on git evidence: "+err.Error())
	default:
		s.merged = tips
	}
	return s.merged
}
