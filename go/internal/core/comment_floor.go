package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func commentFloorFailures(ctx context.Context, in ReviewInput) []string {
	stage := commentFloorStage(in.ProjectRoot)
	if stage == config.StageOff || in.Worktree == "" {
		return nil
	}
	base := floorBase(in)
	paths := changedWorktreePathsSince(ctx, in.Worktree, base, "--no-renames")
	added, err := commentaudit.AddedAcrossDiff(paths, commentaudit.ReadAtBase(worktreeGit(ctx, in.Worktree), base), worktreeFile(in.Worktree))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN comment floor could not read the diff (%v); the audit still judges comments\n", err)
		return nil
	}
	failures, warning := commentFloorResult(stage, added)
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	return failures
}

func commentFloorResult(stage config.Stage, added []commentaudit.Added) (failures []string, warning string) {
	if stage == config.StageOff || len(added) == 0 {
		return nil, ""
	}
	lines := make([]string, 0, len(added))
	for _, c := range added {
		lines = append(lines, c.File+": "+c.Line)
	}
	listing := strings.Join(lines, "\n    ")
	if stage != config.StageEnforce {
		return nil, fmt.Sprintf("[build-floor] WARN comment floor (shadow): the build adds %d comment line(s):\n    %s", len(added), listing)
	}
	return []string{fmt.Sprintf("the build adds %d comment line(s); new and changed code carries no comments (docs/conventions/code-comments.md). Delete each one; put what it says into a name, a test name or the package's design notes:\n    %s", len(added), listing)}, ""
}

func commentFloorStage(projectRoot string) config.Stage {
	word := policy.Policy{}.CommentFloorConfig().Stage
	if projectRoot != "" {
		if p, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
			word = p.CommentFloorConfig().Stage
		}
	}
	stage, ok := config.GateStage(word)
	if !ok {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN comment_floor.stage %q is not off, shadow or enforce; the comment floor is off\n", word)
	}
	return stage
}

func floorBase(in ReviewInput) string {
	if in.WorktreeBaseSHA == "" {
		return "HEAD"
	}
	return in.WorktreeBaseSHA
}

func worktreeGit(ctx context.Context, worktree string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		out, stderr, code, err := gitexec.Git{Dir: worktree, Exec: gitRunner}.Capture(ctx, args...)
		if err != nil || code != 0 {
			return nil, fmt.Errorf("git %s: rc=%d: %v: %s", strings.Join(args, " "), code, err, strings.TrimSpace(stderr))
		}
		return []byte(out), nil
	}
}

func worktreeFile(worktree string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(worktree, filepath.FromSlash(path)))
	}
}
