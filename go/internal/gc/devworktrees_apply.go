package gc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

var errBudgetDeferred = errors.New("gc: the time budget ran out")

func ApplyDevWorktrees(ctx context.Context, o DevOptions, m DevManifest) ([]DevArchiveRecord, error) {
	s, skipped, err := newDevSweep(ctx, o)
	if err != nil || skipped != "" {
		return nil, err
	}
	s.strictPR = true
	trees, err := s.hub.DevWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]wtcheckpoint.Worktree, len(trees))
	for _, wt := range trees {
		known[resolvePath(wt.Dir)] = wt
	}
	var done []DevArchiveRecord
	var errs []error
	for _, planned := range m.Items {
		if planned.Action != DevActionArchive {
			continue
		}
		rec, err := s.archiveOne(ctx, planned, known)
		if rec.Path != "" {
			done = append(done, rec)
		}
		if errors.Is(err, errBudgetDeferred) {
			o.deferred(planned.Path)
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return done, errors.Join(errs...)
}

func (s *devSweep) archiveOne(ctx context.Context, planned DevItem, known map[string]wtcheckpoint.Worktree) (DevArchiveRecord, error) {
	wt, ok := known[resolvePath(planned.Path)]
	if !ok {
		return DevArchiveRecord{}, fmt.Errorf("gc: refuse %s: not a dev worktree of the hub", planned.Path)
	}
	fresh := s.judge(ctx, wt)
	switch {
	case fresh.Skipped:
		return DevArchiveRecord{}, errBudgetDeferred
	case fresh.Action != DevActionArchive:
		return DevArchiveRecord{}, fmt.Errorf("gc: refuse %s: %s since the plan", planned.Path, fresh.Reason)
	case fresh.Head != planned.Head:
		return DevArchiveRecord{}, fmt.Errorf("gc: refuse %s: HEAD moved from %s to %s since the plan", planned.Path, planned.Head, fresh.Head)
	}
	rec := DevArchiveRecord{Path: wt.Dir, Branch: fresh.Branch, Head: fresh.Head, IdleDays: fresh.IdleDays, ArchivedAt: s.now}
	removal := []string{"worktree", "remove"}
	if fresh.Dirty {
		ref, err := s.saveVerified(ctx, wt)
		if err != nil {
			return DevArchiveRecord{}, err
		}
		rec.Checkpoint = ref
		removal = append(removal, "--force")
	}
	if err := s.store.Run(ctx, append(removal, filepath.Clean(wt.Dir))...); err != nil {
		return DevArchiveRecord{}, fmt.Errorf("gc: archive %s: %w", wt.Dir, err)
	}
	if err := AppendDevArchive(s.o.EvolveDir, rec); err != nil {
		return rec, fmt.Errorf("gc: %s was archived but its record was not written (%+v): %w", wt.Dir, rec, err)
	}
	return rec, nil
}

func (s *devSweep) saveVerified(ctx context.Context, wt wtcheckpoint.Worktree) (string, error) {
	tree := gitexec.Git{Dir: wt.Dir, Exec: s.o.Exec}
	before, err := tree.DirtyPaths(ctx)
	if err != nil {
		return "", fmt.Errorf("gc: refuse %s: dirty check failed: %w", wt.Dir, err)
	}
	saved, err := s.o.save(ctx, wt)
	if err != nil {
		return "", fmt.Errorf("gc: refuse %s: checkpoint save failed, so the work stays in place: %w", wt.Dir, err)
	}
	after, err := tree.DirtyPaths(ctx)
	if err != nil {
		return "", fmt.Errorf("gc: refuse %s: dirty re-check failed: %w", wt.Dir, err)
	}
	again, err := s.o.save(ctx, wt)
	if err != nil {
		return "", fmt.Errorf("gc: refuse %s: save re-check failed: %w", wt.Dir, err)
	}
	if !slices.Equal(before, after) || again.Ref != saved.Ref {
		return "", fmt.Errorf("gc: refuse %s: the tree changed while its checkpoint was saved, so it is kept and not archived", wt.Dir)
	}
	return saved.Ref, nil
}
