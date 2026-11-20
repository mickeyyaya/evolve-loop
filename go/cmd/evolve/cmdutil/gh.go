package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func GHIn(root string) func(ctx context.Context, args ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := sysexec.Command(ctx, "gh", args...)
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
}

func PlaneLiveRuns(root string, now time.Time) ([]runlease.LiveRun, error) {
	out, err := sysexec.Command(context.Background(), "git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list in %s: %w", root, err)
	}
	trees := []string{root}
	for _, line := range strings.Split(string(out), "\n") {
		if tree, ok := strings.CutPrefix(line, "worktree "); ok {
			trees = append(trees, tree)
		}
	}
	var live []runlease.LiveRun
	seen := map[string]bool{}
	for _, tree := range trees {
		runs := filepath.Join(paths.EvolveDirOf(tree), "runs")
		if real, err := filepath.EvalSymlinks(runs); err == nil {
			runs = real
		}
		if seen[runs] {
			continue
		}
		seen[runs] = true
		live = append(live, runlease.LiveRuns(runs, now)...)
	}
	return live, nil
}
