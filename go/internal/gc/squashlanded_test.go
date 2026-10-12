package gc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func writeRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitRepoFile(t *testing.T, r *gittest.Repo, rel, body string) {
	t.Helper()
	writeRepoFile(t, r.Dir, rel, body)
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "change "+rel)
}

func newSquashRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	commitRepoFile(t, r, "base.txt", "base\n")
	return r
}

func squashMerge(r *gittest.Repo, branch string) {
	r.Git("switch", "-q", "main")
	r.Git("merge", "--squash", branch)
	r.Git("commit", "-q", "-m", "squash "+branch)
}

func realGit(dir string) gitexec.Git { return gitexec.Git{Dir: dir, Exec: sysexec.DefaultRunner} }

func TestBranchLandedBySquash_ASquashMergedBranchIsLandedEvenThoughItIsNotAnAncestor(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "a.txt", "a\n")
	commitRepoFile(t, r, "dir/b.txt", "b\n")
	squashMerge(r, "feature")
	commitRepoFile(t, r, "later.txt", "unrelated later work on main\n")

	if got := r.Git("branch", "--merged", "HEAD", "--list", "feature"); got != "" {
		t.Fatalf("premise broken: feature is an ancestor of main (%q); a squash merge must leave it unmerged by ancestry", got)
	}
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "HEAD", "feature")
	if err != nil || !landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want true, nil for a squash-merged branch", landed, err)
	}
}

func TestBranchLandedBySquash_AnExtraCommitAfterTheSquashIsNotLanded(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "a.txt", "a\n")
	squashMerge(r, "feature")
	r.Git("switch", "-q", "feature")
	commitRepoFile(t, r, "after.txt", "work done after the squash\n")

	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want false, nil when the branch holds a change main lacks", landed, err)
	}
}

func TestBranchLandedBySquash_ABranchWhoseFileMainChangedAgainIsKept(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "a.txt", "v1\n")
	squashMerge(r, "feature")
	commitRepoFile(t, r, "a.txt", "v2\n")

	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want false, nil (conservative) when main edited the landed file again", landed, err)
	}
}

func TestBranchLandedBySquash_ADeletedFileCountsAsLandedOnlyWhenMainDeletedItToo(t *testing.T) {
	r := newSquashRepo(t)
	commitRepoFile(t, r, "gone.txt", "to delete\n")
	r.Git("switch", "-q", "-c", "feature")
	r.Git("rm", "-q", "gone.txt")
	r.Git("commit", "-q", "-m", "delete gone")
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || landed {
		t.Fatalf("before the squash: landed = %v, %v; want false, nil (main still has the file)", landed, err)
	}
	squashMerge(r, "feature")
	landed, err = landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || !landed {
		t.Fatalf("after the squash: landed = %v, %v; want true, nil (both sides deleted the file)", landed, err)
	}
}

func TestBranchLandedBySquash_ABranchWithNoChangesIsLanded(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("branch", "same")
	commitRepoFile(t, r, "main-only.txt", "x\n")
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "same")
	if err != nil || !landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want true, nil for a branch with nothing to land", landed, err)
	}
}

func TestBranchLandedBySquash_AManyFileSquashIsComparedAcrossEveryPathspecChunk(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	total := squashPathspecChunk*2 + 30
	for i := 0; i < total; i++ {
		writeRepoFile(t, r.Dir, fmt.Sprintf("many/f%03d.txt", i), fmt.Sprintf("v%d\n", i))
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "many files")
	squashMerge(r, "feature")
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || !landed {
		t.Fatalf("exact squash of %d files: landed = %v, %v; want true, nil", total, landed, err)
	}
	commitRepoFile(t, r, fmt.Sprintf("many/f%03d.txt", total-1), "edited in the last chunk\n")
	landed, err = landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || landed {
		t.Fatalf("a difference in the last chunk: landed = %v, %v; want false, nil (every chunk must be compared)", landed, err)
	}
}

func TestBranchLandedBySquash_APathNamedLikeAGlobIsMatchedLiterally(t *testing.T) {
	r := newSquashRepo(t)
	commitRepoFile(t, r, "other.txt", "v1\n")
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "*.txt", "the literal star file\n")
	squashMerge(r, "feature")
	commitRepoFile(t, r, "other.txt", "v2: main moved on in a file the branch never touched\n")

	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "feature")
	if err != nil || !landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want true, nil: the branch changed only the literal path '*.txt', and other.txt must not be compared as a glob match", landed, err)
	}
}

func TestBranchLandedBySquash_NoCommonAncestorIsNotLanded(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "--orphan", "island")
	commitRepoFile(t, r, "island.txt", "unrelated history\n")
	landed, err := landedBySquash(context.Background(), realGit(r.Dir), "main", "island")
	if err != nil || landed {
		t.Fatalf("branchLandedBySquash = %v, %v; want false, nil for unrelated histories", landed, err)
	}
}

func TestBranchLandedBySquash_GitFailuresSurfaceAsErrors(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "feature")
	commitRepoFile(t, r, "a.txt", "a\n")
	boom := errors.New("git vanished")
	cases := []struct {
		name   string
		failOn string
		code   int
		err    error
	}{
		{"merge-base cannot run", "merge-base", 0, boom},
		{"merge-base exits 128", "merge-base", 128, nil},
		{"diff of names cannot run", "--name-only", 0, boom},
		{"diff of names exits 128", "--name-only", 128, nil},
		{"pathspec diff cannot run", "--quiet", 0, boom},
		{"pathspec diff exits 128", "--quiet", 128, nil},
	}
	for _, c := range cases {
		run := func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
			for _, a := range args {
				if a == c.failOn {
					return c.code, c.err
				}
			}
			return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
		}
		landed, err := landedBySquash(context.Background(), gitexec.Git{Dir: r.Dir, Exec: run}, "main", "feature")
		if err == nil || landed {
			t.Errorf("%s: branchLandedBySquash = %v, %v; want false and an error", c.name, landed, err)
		}
	}
}

func landedBySquash(ctx context.Context, g gitexec.Git, target, branch string) (bool, error) {
	res, err := branchLandedBySquash(ctx, g, target, branch)
	return res.landing() != landingNone, err
}
