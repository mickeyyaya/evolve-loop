//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func failingRunnerFor(marker string) CmdRunner {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if strings.Contains(strings.Join(args, " "), marker) {
			return 0, errors.New("injected git failure")
		}
		return execRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func TestStageablePaths_UnstagedDeletionIsKept(t *testing.T) {
	t.Parallel()
	repo := manualPathsRepo(t)
	if err := os.Remove(filepath.Join(repo, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	ctx := WithPaths(context.Background(), []string{"doomed.txt"})

	keep, err := stageablePaths(ctx, &Options{ProjectRoot: repo, Runner: execRunner})

	if err != nil || len(keep) != 1 || keep[0] != "doomed.txt" {
		t.Fatalf("an unstaged deletion must be kept: keep=%v err=%v", keep, err)
	}
}

func TestStageablePaths_GitReadFailuresSurface(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{"ls-files", "diff --cached"} {
		repo := manualPathsRepo(t)
		ctx := WithPaths(context.Background(), []string{"ghost-never-existed.txt"})

		keep, err := stageablePaths(ctx, &Options{ProjectRoot: repo, Runner: failingRunnerFor(marker)})

		if err == nil || keep != nil {
			t.Errorf("%s failure must surface: keep=%v err=%v", marker, keep, err)
		}
	}
}
