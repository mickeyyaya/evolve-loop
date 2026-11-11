package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestFloorEntryRun_BuildsTheReviewInputFromTheRootAndTheBase(t *testing.T) {
	t.Parallel()
	var got ReviewInput
	run := FloorEntryRun(func(_ context.Context, in ReviewInput) ([]string, error) {
		got = in
		return nil, nil
	})
	root := t.TempDir()
	if _, err := run(context.Background(), gatecontract.Input{Root: root, Base: "abc"}, gatecontract.Ports{}); err != nil {
		t.Fatal(err)
	}
	if got.Worktree != root || got.ProjectRoot != root || got.WorktreeBaseSHA != "abc" {
		t.Errorf("review input = worktree %q, project %q, base %q, want the root twice and the base", got.Worktree, got.ProjectRoot, got.WorktreeBaseSHA)
	}
}

func TestFloorEntryRun_EachJudgeLineIsOneRed(t *testing.T) {
	t.Parallel()
	run := FloorEntryRun(func(context.Context, ReviewInput) ([]string, error) { return []string{"one", "two\nmore"}, nil })
	verdict, err := run(context.Background(), gatecontract.Input{Root: t.TempDir()}, gatecontract.Ports{})
	if err != nil || len(verdict.Reds) != 2 || verdict.Reds[1] != "two\nmore" {
		t.Errorf("Run = %+v, %v, want the two judge lines as two reds and no error", verdict, err)
	}
}

func TestFloorEntryRun_AJudgeErrorIsAnErrorAndNeitherAPassNorARed(t *testing.T) {
	t.Parallel()
	cause := errors.New("no read")
	run := FloorEntryRun(func(context.Context, ReviewInput) ([]string, error) { return []string{"red"}, cause })
	verdict, err := run(context.Background(), gatecontract.Input{Root: t.TempDir()}, gatecontract.Ports{})
	if !errors.Is(err, cause) || len(verdict.Reds) != 0 {
		t.Errorf("Run = %+v, %v, want the judge error and an empty verdict", verdict, err)
	}
}

func TestFloorEntryRun_AnUnreadableRootIsAnErrorAndTheJudgeNeverRuns(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ran := false
	run := FloorEntryRun(func(context.Context, ReviewInput) ([]string, error) { ran = true; return nil, nil })
	for name, root := range map[string]string{"missing": file + "-absent", "a file": file} {
		if _, err := run(context.Background(), gatecontract.Input{Root: root}, gatecontract.Ports{}); err == nil {
			t.Errorf("%s: Run = nil error, want an error", name)
		}
	}
	if ran {
		t.Error("the judge ran over an unreadable root")
	}
}

func TestFloorPaths_ReadsTheChangeAndErrorsOnAnUnreadableBase(t *testing.T) {
	t.Parallel()
	if paths, err := floorPaths(context.Background(), ReviewInput{}); err != nil || paths != nil {
		t.Errorf("floorPaths with no worktree = %v, %v, want none", paths, err)
	}
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "old.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo.Dir, "new.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	if paths, err := floorPaths(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: head}); err != nil || len(paths) != 1 || paths[0] != "new.go" {
		t.Errorf("floorPaths = %v, %v, want new.go", paths, err)
	}
	if _, err := floorPaths(context.Background(), ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: strings.Repeat("0", 40)}); err == nil {
		t.Error("floorPaths with an unknown base = nil error, want an error")
	}
}

func TestFloorJudges_AnUnreadableBaseIsAnErrorForTheUnitAndCoverJudges(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "base")
	in := ReviewInput{Worktree: repo.Dir, ProjectRoot: repo.Dir, WorktreeBaseSHA: strings.Repeat("0", 40)}
	for name, judge := range map[string]FloorJudge{"unit": UnitTestFloorJudge, "cover": CoverChangedFloorJudge} {
		if lines, err := judge(context.Background(), in); err == nil || len(lines) != 0 {
			t.Errorf("%s judge = %v, %v, want an error and no line", name, lines, err)
		}
	}
}

func TestWorktreePathsSince_AFailedDiffIsAnErrorAndKeepsTheUntrackedPaths(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo.Dir, "new.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unknown := strings.Repeat("0", 40)
	paths, err := worktreePathsSince(context.Background(), repo.Dir, unknown)
	if err == nil || !strings.Contains(err.Error(), "git diff") || len(paths) != 1 || paths[0] != "new.go" {
		t.Errorf("worktreePathsSince = %v, %v, want new.go and an error that names git diff", paths, err)
	}
	if got := changedWorktreePathsSince(context.Background(), repo.Dir, unknown); len(got) != 1 || got[0] != "new.go" {
		t.Errorf("changedWorktreePathsSince = %v, want new.go", got)
	}
}
