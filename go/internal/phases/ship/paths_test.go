package ship

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestPreviewStagedSet_ListsEveryChangedPathAndLeavesTheIndexAlone(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	writeDoc(t, root, "a.go", "package a\n")
	writeDoc(t, root, "pkg/b.go", "package b\n")
	opts := &Options{Class: ClassManual, DryRun: true, ProjectRoot: root, Runner: execRunner}
	res := &RunResult{}

	if err := verifyClass(context.Background(), opts, res); err != nil {
		t.Fatalf("dry-run preview must not fail: %v", err)
	}

	for _, want := range []string{"[DRY-RUN] staged set: a.go", "[DRY-RUN] staged set: pkg/b.go"} {
		if !containsLog(*res, want) {
			t.Errorf("missing %q in %v", want, res.Logs)
		}
	}
	staged, err := exec.Command("git", "-C", root, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(staged)) != "" {
		t.Errorf("dry-run staged %q, want an untouched index", staged)
	}
}

func TestVerifyClass_ManualDryRunOnACleanTreeIsAnEmptyDiffExit(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	opts := &Options{Class: ClassManual, DryRun: true, ProjectRoot: root, Runner: execRunner}

	err := verifyClass(context.Background(), opts, &RunResult{})

	if !errors.Is(err, errEmptyDiff) {
		t.Fatalf("clean tree dry-run = %v, want errEmptyDiff", err)
	}
}

func TestPreviewStagedSet_ReportsAGitFailure(t *testing.T) {
	t.Parallel()
	opts := &Options{Class: ClassManual, DryRun: true, ProjectRoot: filepath.Join(gittest.Fixture(t).Dir, "missing"), Runner: execRunner}

	if err := previewStagedSet(context.Background(), opts, &RunResult{}); err == nil {
		t.Fatal("a failing git status must surface as an error")
	}
}

func TestVerifyClass_CycleStopsWhenTheChangedPathsCannotBeRead(t *testing.T) {
	t.Parallel()
	opts := &Options{Class: ClassCycle, ProjectRoot: filepath.Join(t.TempDir(), "absent"), Runner: execRunner}

	if err := verifyClass(context.Background(), opts, &RunResult{}); err == nil {
		t.Fatal("an unreadable tree must stop the cycle verify before the audit binding")
	}
}

func TestScopeStatusRead_AppendsThePathspecToTheStagedSetReadOnly(t *testing.T) {
	t.Parallel()
	scoped := WithPaths(context.Background(), []string{"a.go", "pkg/"})
	status := []string{"-c", "core.quotePath=false", "status", "--porcelain", "-uall"}
	cases := []struct {
		name string
		ctx  context.Context
		args []string
		want string
	}{
		{"status-read", scoped, status, "-c core.quotePath=false status --porcelain -uall -- a.go pkg/"},
		{"other-read", scoped, []string{"rev-parse", "HEAD"}, "rev-parse HEAD"},
		{"porcelain-without-uall", scoped, []string{"status", "--porcelain"}, "status --porcelain"},
		{"no-paths", context.Background(), status, "-c core.quotePath=false status --porcelain -uall"},
		{"empty-paths", WithPaths(context.Background(), nil), status, "-c core.quotePath=false status --porcelain -uall"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(scopeStatusRead(tc.ctx, tc.args), " "); got != tc.want {
				t.Errorf("scopeStatusRead = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStageArgs_NamesTheScopeOnlyWhenPathsAreCarried(t *testing.T) {
	t.Parallel()
	if got := strings.Join(stageArgs(context.Background()), " "); got != "add -A" {
		t.Errorf("stageArgs without paths = %q, want whole-tree add", got)
	}
	if got := strings.Join(stageArgs(WithPaths(context.Background(), []string{"a.go"})), " "); got != "add -A -- a.go" {
		t.Errorf("stageArgs with paths = %q", got)
	}
}

func TestPreviewStagedSet_PathsRestrictTheListedSet(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	writeDoc(t, root, "a.go", "package a\n")
	writeDoc(t, root, "b.go", "package b\n")
	opts := &Options{Class: ClassManual, DryRun: true, ProjectRoot: root, Runner: execRunner}
	res := &RunResult{}

	if err := verifyClass(WithPaths(context.Background(), []string{"a.go"}), opts, res); err != nil {
		t.Fatal(err)
	}

	if !containsLog(*res, "staged set: a.go") || containsLog(*res, "b.go") {
		t.Errorf("logs = %v, want only a.go", res.Logs)
	}
}
