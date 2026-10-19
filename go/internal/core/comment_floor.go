package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var unboundTddPredicateDir = regexp.MustCompile(`^go/acs/cycle[0-9]+/`)

func commentFloorFailures(ctx context.Context, in ReviewInput) []string {
	stage := commentFloorStage(in.ProjectRoot, in.errOut())
	if stage == config.StageOff || in.Worktree == "" {
		return nil
	}
	base := floorBase(in)
	paths := withoutTddOwnedPredicates(changedWorktreePathsSince(ctx, in.Worktree, base, "--no-renames"), in.Cycle)
	added, err := commentaudit.AddedAcrossDiff(paths, commentaudit.ReadAtBase(worktreeGit(ctx, in.Worktree), base), worktreeFile(in.Worktree))
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN comment floor could not read the diff (%v); the audit still judges comments\n", err)
		return nil
	}
	failures, warning := commentFloorResult(stage, added)
	if warning != "" {
		fmt.Fprintln(in.errOut(), warning)
	}
	return failures
}

func withoutTddOwnedPredicates(paths []string, cycle int) []string {
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if !isTddOwnedPredicate(p, cycle) {
			kept = append(kept, p)
		}
	}
	return kept
}

func isTddOwnedPredicate(path string, cycle int) bool {
	if cycle > 0 {
		return strings.HasPrefix(path, fmt.Sprintf("go/acs/cycle%d/", cycle))
	}
	return unboundTddPredicateDir.MatchString(path)
}

func TddCommentFloorFailures(ctx context.Context, worktree, projectRoot string, files []string, warn io.Writer) []string {
	if commentFloorStage(projectRoot, warn) == config.StageOff || worktree == "" || len(files) == 0 {
		return nil
	}
	added, err := commentaudit.AddedAcrossDiff(files, commentaudit.ReadAtBase(worktreeGit(ctx, worktree), "HEAD"), worktreeFile(worktree))
	if err != nil {
		fmt.Fprintf(warn, "[tdd-floor] WARN comment floor could not read the predicate files (%v)\n", err)
		return nil
	}
	lines := make([]string, 0, len(added))
	for _, c := range added {
		lines = append(lines, c.File+": "+c.Line)
	}
	return lines
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

func commentFloorStage(projectRoot string, warn io.Writer) config.Stage {
	return floorStage(projectRoot, "comment_floor", "comment floor", func(p policy.Policy) string { return p.CommentFloorConfig().Stage }, warn)
}

func floorStage(projectRoot, key, label string, word func(policy.Policy) string, warn io.Writer) config.Stage {
	chosen := word(policy.Policy{})
	if projectRoot != "" {
		if p, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
			chosen = word(p)
		}
	}
	stage, ok := config.GateStage(chosen)
	if !ok {
		fmt.Fprintf(warn, "[build-floor] WARN %s.stage %q is not off, shadow or enforce; the %s is off\n", key, chosen, label)
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
