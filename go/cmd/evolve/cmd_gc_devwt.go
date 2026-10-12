package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const ghOpenPRLimit = "1000"

func (r gcRun) devWorktrees(projectRoot, evolveDir string, pol gc.Policy) bool {
	opts, err := devWorktreeGCOptions(projectRoot, evolveDir, pol)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: dev worktree sweep refused: %v\n", err)
		return true
	}
	return r.devWorktreesWith(opts)
}

func (r gcRun) devWorktreesWith(opts gc.DevOptions) bool {
	budget := opts.Policy.SweepBudget()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), budget)
	defer cancel()
	manifest, err := gc.PlanDevWorktrees(ctx, opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: dev worktree sweep plan failed: %v\n", err)
		return true
	}
	if manifest.OverBudget {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: dev worktree sweep over its %s budget (gc.dev_worktrees.sweep_budget_seconds): %s\n", budget, manifest.Skipped)
		return false
	}
	if manifest.Skipped != "" {
		fmt.Fprintf(r.stdout, "evolve gc: dev worktree sweep skipped: %s\n", manifest.Skipped)
		return false
	}
	archive, kept, unjudged := gcDevCounts(manifest)
	r.summary("%d dev worktree(s) would be archived (%d kept, %d not judged)", "applying dev worktree archive — %d item(s) (%d kept, %d not judged)", archive, kept, unjudged)
	for _, n := range manifest.Notes {
		fmt.Fprintf(r.stdout, "  NOTE %s\n", n)
	}
	for _, it := range manifest.Items {
		fmt.Fprintf(r.stdout, "  %s\n", gcDevItemLine(it, r.dryRun))
	}
	failed := unjudged > 0
	if skipped := gcDevSkipped(manifest); skipped > 0 {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: dev worktree sweep used its %s budget (gc.dev_worktrees.sweep_budget_seconds): %d tree(s) SKIPPED, not judged\n", budget, skipped)
	}
	if r.dryRun {
		return failed
	}
	if ctx.Err() != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: dev worktree archive deferred to the next run: the %s budget is spent\n", budget)
		return failed
	}
	return r.applyDevArchive(ctx, opts, manifest, budget, failed)
}

func (r gcRun) applyDevArchive(ctx context.Context, opts gc.DevOptions, manifest gc.DevManifest, budget time.Duration, failed bool) bool {
	opts.Deferred = func(path string) {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: dev worktree %s kept: the %s budget ran out before its archive; the next run retries\n", path, budget)
	}
	recs, err := gc.ApplyDevWorktrees(ctx, opts, manifest)
	for _, rec := range recs {
		fmt.Fprintf(r.stdout, "  archived path=%s branch=%s checkpoint=%s\n", rec.Path, rec.Branch, gcCheckpointLabel(rec))
		for _, cmd := range rec.RestoreCommands() {
			fmt.Fprintf(r.stdout, "    restore: %s\n", cmd)
		}
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: dev worktree archive partial: %v\n", err)
		return true
	}
	fmt.Fprintf(r.stdout, "evolve gc: dev worktree archive applied; manifest %s\n", gc.DevArchivePath(opts.EvolveDir))
	return failed
}

func gcCheckpointLabel(rec gc.DevArchiveRecord) string {
	if rec.Checkpoint == "" {
		return "none (the tree was clean)"
	}
	return rec.Checkpoint
}

func gcDevItemLine(it gc.DevItem, dryRun bool) string {
	verb := "KEEP"
	switch {
	case it.Failed:
		verb = "NOT-JUDGED"
	case it.Skipped:
		verb = "SKIPPED"
	case it.Action == gc.DevActionArchive && dryRun:
		verb = "WOULD-ARCHIVE"
	case it.Action == gc.DevActionArchive:
		verb = "ARCHIVE"
	}
	label := "path=" + it.Path
	if it.Branch != "" {
		label += " branch=" + it.Branch
	}
	return fmt.Sprintf("%s %s idle=%.1fd (%s)", verb, label, it.IdleDays, it.Reason)
}

func gcDevCounts(m gc.DevManifest) (archive, kept, unjudged int) {
	for _, it := range m.Items {
		switch {
		case it.Failed:
			unjudged++
		case it.Skipped:
		case it.Action == gc.DevActionArchive:
			archive++
		default:
			kept++
		}
	}
	return archive, kept, unjudged
}

func gcDevSkipped(m gc.DevManifest) int {
	n := 0
	for _, it := range m.Items {
		if it.Skipped {
			n++
		}
	}
	return n
}

type ghOpenPR struct {
	HeadRefName string    `json:"headRefName"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func parseOpenPRUpdates(raw []byte) (map[string][]time.Time, error) {
	var prs []ghOpenPR
	if err := json.Unmarshal(raw, &prs); err != nil {
		return nil, fmt.Errorf("gh pr list: unreadable answer: %w", err)
	}
	byBranch := map[string][]time.Time{}
	for _, pr := range prs {
		byBranch[pr.HeadRefName] = append(byBranch[pr.HeadRefName], pr.UpdatedAt)
	}
	return byBranch, nil
}

func ghOpenPRUpdates(ctx context.Context, dir string) (map[string][]time.Time, error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", gc.ErrPRLookupUnavailable, err)
	}
	ctx, cancel := context.WithTimeout(ctx, devGHTimeout)
	defer cancel()
	cmd := sysexec.Command(ctx, gh, "pr", "list", "--state", "open", "--limit", ghOpenPRLimit, "--json", "headRefName,updatedAt")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w: %s", err, strings.TrimSpace(ghStderr(err)))
	}
	return parseOpenPRUpdates(out)
}

func ghStderr(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(exitErr.Stderr)
	}
	return ""
}
