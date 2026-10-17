package gc

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const landedInMain = "is in main"

func (s *devSweep) landedEarly(ctx context.Context, it *DevItem) (bool, error) {
	tree := gitexec.Git{Dir: it.Path, Exec: s.o.Exec}
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
