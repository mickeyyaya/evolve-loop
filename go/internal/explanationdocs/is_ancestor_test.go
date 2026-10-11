package explanationdocs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func twoCommitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	r := gittest.Fixture(t)
	for i, body := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(r.Dir, "f.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r.Git("add", "f.txt")
		r.Git("commit", "-q", "-m", body)
		sha := r.Git("rev-parse", "HEAD")
		if i == 0 {
			first = sha
		} else {
			second = sha
		}
	}
	return r.Dir, first, second
}

func TestIsAncestor_AnswersTheCommitGraph(t *testing.T) {
	dir, first, second := twoCommitRepo(t)
	cases := []struct {
		name                 string
		ancestor, descendant string
		want                 bool
	}{
		{"parent of child", first, second, true},
		{"child of parent", second, first, false},
		{"commit of itself", second, second, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isAncestor(context.Background(), dir, tc.ancestor, tc.descendant)

			if err != nil || got != tc.want {
				t.Errorf("isAncestor = (%v, %v), want (%v, nil)", got, err, tc.want)
			}
		})
	}
}

func TestIsAncestor_GitFailuresAreErrorsNotAnswers(t *testing.T) {
	dir, first, _ := twoCommitRepo(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name     string
		ctx      context.Context
		worktree string
		ancestor string
		wantText string
	}{
		{"unknown revision", context.Background(), dir, "0123456789abcdef0123456789abcdef01234567", "git merge-base --is-ancestor 0123456789abcdef0123456789abcdef01234567 " + first},
		{"not a repository", context.Background(), t.TempDir(), first, "not a git repository"},
		{"cancelled context", cancelled, dir, first, "git merge-base --is-ancestor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isAncestor(tc.ctx, tc.worktree, tc.ancestor, first)

			if err == nil || got {
				t.Fatalf("isAncestor = (%v, %v), want (false, error)", got, err)
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("err = %v, want it to contain %q", err, tc.wantText)
			}
		})
	}
}
