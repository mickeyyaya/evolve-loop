package gc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const (
	landedInMain   = "is in main"
	landedClosedPR = "has a closed pull request"
)

func (s *devSweep) landedEarly(ctx context.Context, it *DevItem) (bool, string, error) {
	tree := gitexec.Git{Dir: it.Path, Exec: s.o.Exec}
	dirty, err := tree.DirtyPaths(ctx)
	if err != nil {
		return false, "", err
	}
	if len(dirty) > 0 {
		it.Dirty = true
		return false, "dirty: " + strings.Join(dirty, ", "), nil
	}
	if it.Branch == "" {
		return s.detached[it.Path].merged, "", nil
	}
	own, err := hasOwnWork(ctx, tree, it.Head)
	if err != nil || !own {
		return false, "", err
	}
	landed, err := s.landed(ctx, *it)
	if err != nil || (landed != landedInMain && landed != landedClosedPR) {
		return false, "", err
	}
	prs, err := s.openPRs(ctx)
	if err != nil || s.prsSkipped {
		return false, "", err
	}
	if updates := prs[it.Branch]; len(updates) > 0 {
		return false, openPRCause(s.now, updates), nil
	}
	return true, "", nil
}

func openPRCause(now time.Time, updates []time.Time) string {
	newest := updates[0]
	for _, u := range updates[1:] {
		if u.After(newest) {
			newest = u
		}
	}
	return fmt.Sprintf("open pull request updated %.1f days ago", now.Sub(newest).Hours()/24)
}

func hasOwnWork(ctx context.Context, tree gitexec.Git, head string) (bool, error) {
	return branchHasOwnWork(ctx, tree, "HEAD", head)
}

func branchHasOwnWork(ctx context.Context, g gitexec.Git, ref, tip string) (bool, error) {
	out, err := g.Output(ctx, "reflog", "show", "--format=%H", ref)
	if err != nil {
		return false, fmt.Errorf("git reflog show %s: %w", ref, err)
	}
	lines := strings.Fields(out)
	if len(lines) == 0 {
		return false, nil
	}
	return lines[len(lines)-1] != tip, nil
}

func (s *devSweep) mergedTips(ctx context.Context) map[string][]string {
	return s.prTips(ctx, "merged", &s.mergedRead, &s.merged, s.o.MergedPRs)
}

func (s *devSweep) closedTips(ctx context.Context) map[string][]string {
	return s.prTips(ctx, "closed", &s.closedRead, &s.closed, s.o.ClosedPRs)
}

func (s *devSweep) prTips(ctx context.Context, kind string, read *bool, cache *map[string][]string, lookup MergedPRLookup) map[string][]string {
	if *read || lookup == nil {
		return *cache
	}
	*read = true
	tips, err := lookup(ctx, s.o.ProjectRoot)
	switch {
	case errors.Is(err, ErrPRLookupUnavailable):
		s.notes = append(s.notes, "gh is unavailable: the "+kind+" pull request source was skipped, so landed rests on git evidence")
	case err != nil:
		s.notes = append(s.notes, "the "+kind+" pull request source failed, so landed rests on git evidence: "+err.Error())
	default:
		*cache = tips
	}
	return *cache
}
