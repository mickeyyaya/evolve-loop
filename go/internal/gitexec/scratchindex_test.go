package gitexec_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func scratchFixture(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.Fixture(t)
	for name, body := range map[string]string{"a.txt": "a\n", ".gitignore": "*.log\n"} {
		if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "base")
	return r, r.Git("rev-parse", "--path-format=absolute", "--git-path", "index")
}

func TestTreeOfIndexCopy_WritesTheStagedAndTheFullTreeWithoutTouchingTheIndex(t *testing.T) {
	r, index := scratchFixture(t)
	for name, body := range map[string]string{"staged.txt": "s\n", "untracked.txt": "u\n", "noise.log": "ignored\n"} {
		if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.Git("add", "staged.txt")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, _ := os.Stat(index)

	for name, g := range map[string]gitexec.Git{"Default": gitexec.Default(r.Dir), "Isolated": gitexec.Isolated(r.Dir)} {
		staged, err := g.TreeOfIndexCopy(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		full, err := g.TreeOfIndexCopy(context.Background(), []string{"add", "-A", "--", ".", ":!a.txt"})
		if err != nil {
			t.Fatal(err)
		}

		after, _ := os.ReadFile(index)
		afterInfo, _ := os.Stat(index)
		if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
			t.Errorf("%s: the real index changed: the tree must be written through a copy", name)
		}
		if got := r.Git("ls-tree", "--name-only", staged); got != ".gitignore\na.txt\nstaged.txt" {
			t.Errorf("%s: staged tree = %q, want what is staged only", name, got)
		}
		if got := r.Git("ls-tree", "--name-only", full); got != ".gitignore\na.txt\nstaged.txt\nuntracked.txt" {
			t.Errorf("%s: full tree = %q, want untracked files in and ignored files out", name, got)
		}
	}
}

func TestTreeOfIndexCopy_IgnoresAmbientGitDirAndIndexFile(t *testing.T) {
	r, _ := scratchFixture(t)
	decoy := gittest.Fixture(t)
	decoy.Git("commit", "-q", "--allow-empty", "-m", "decoy")
	headTree := r.Git("rev-parse", "HEAD^{tree}")
	wantHead := r.Git("rev-parse", "HEAD")
	t.Setenv("GIT_DIR", filepath.Join(decoy.Dir, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy.Dir)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "decoy-index"))

	got, err := gitexec.Default(r.Dir).TreeOfIndexCopy(context.Background())
	full, fullErr := gitexec.Default(r.Dir).TreeOfIndexCopy(context.Background(), []string{"add", "-A", "--", "."})

	if err != nil || got != headTree {
		t.Errorf("TreeOfIndexCopy under ambient git variables = %s, %v; want the worktree's own tree %s", got, err, headTree)
	}
	if fullErr != nil || full != headTree {
		t.Errorf("TreeOfIndexCopy(add -A) under an ambient GIT_WORK_TREE = %s, %v; want the clean worktree's own tree %s, not the decoy work tree", full, fullErr, headTree)
	}
	head, err := gitexec.Isolated(r.Dir).HEAD(context.Background())
	if err != nil || head != wantHead {
		t.Errorf("Isolated(dir).HEAD under ambient git variables = %s, %v; want the worktree's own HEAD", head, err)
	}
}

func TestTreeOfIndexCopy_FailsLoudlyOnAFailingAdd(t *testing.T) {
	r, _ := scratchFixture(t)
	if _, err := gitexec.Default(r.Dir).TreeOfIndexCopy(context.Background(), []string{"add", "--no-such-flag"}); err == nil {
		t.Error("a failing add returned a tree")
	}
	if _, err := gitexec.Default(t.TempDir()).TreeOfIndexCopy(context.Background()); err == nil {
		t.Error("a directory outside any repository returned a tree")
	}
}
