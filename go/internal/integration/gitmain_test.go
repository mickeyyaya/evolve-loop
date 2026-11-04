package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type gitMainFixture struct {
	origin *gittest.Repo
	work   *gittest.Repo
	main   GitMain
}

func newGitMainFixture(t *testing.T) gitMainFixture {
	t.Helper()
	origin := gittest.Bare(t)
	work := gittest.Clone(t, origin.Dir)
	work.Write("a.txt", "a\n")
	work.Git("add", ".")
	work.Git("commit", "-q", "-m", "one")
	work.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	return gitMainFixture{origin: origin, work: work, main: GitMain{Repo: Repo{Dir: work.Dir, Git: SysGit()}}}
}

func TestGitMainHeadIsTheRemoteMain(t *testing.T) {
	t.Parallel()
	f := newGitMainFixture(t)
	head, err := f.main.Head(context.Background())
	if err != nil || head != headOf(f.work) {
		t.Errorf("Head = %q, %v, want %q", head, err, headOf(f.work))
	}
}

func TestGitMainFastForwardMovesMainToTheTip(t *testing.T) {
	t.Parallel()
	f := newGitMainFixture(t)
	f.work.Write("b.txt", "b\n")
	f.work.Git("add", ".")
	f.work.Git("commit", "-q", "-m", "two")
	tip := headOf(f.work)
	if err := f.main.FastForward(context.Background(), tip); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(f.origin.Git("rev-parse", "refs/heads/main")); got != tip {
		t.Errorf("origin main = %s, want %s", got, tip)
	}
}

func TestGitMainFastForwardRefusesANonFastForward(t *testing.T) {
	t.Parallel()
	f := newGitMainFixture(t)
	f.work.Git("checkout", "-q", "-b", "side", "HEAD~0")
	f.work.Write("c.txt", "c\n")
	f.work.Git("add", ".")
	f.work.Git("commit", "-q", "-m", "side")
	side := headOf(f.work)
	f.work.Git("checkout", "-q", "main")
	f.work.Write("d.txt", "d\n")
	f.work.Git("add", ".")
	f.work.Git("commit", "-q", "-m", "moved")
	f.work.Git("push", "-q", "origin", "HEAD:refs/heads/main")
	if err := f.main.FastForward(context.Background(), side); err == nil {
		t.Error("FastForward of a diverged tip returned nil, want a refusal")
	}
}

func TestGitMainFastForwardRefusesANonObjectID(t *testing.T) {
	t.Parallel()
	if err := (GitMain{}).FastForward(context.Background(), "main"); err == nil {
		t.Error("FastForward of a ref name returned nil")
	}
}

func TestGitMainHeadFailures(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	m := GitMain{Repo: Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "", boom }}}
	if _, err := m.Head(context.Background()); !errors.Is(err, boom) {
		t.Errorf("Head = %v, want boom", err)
	}
	m = GitMain{Repo: Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "", nil }}}
	if _, err := m.Head(context.Background()); err == nil {
		t.Error("Head of an origin with no main returned nil")
	}
	if err := (GitMain{Repo: Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "", boom }}}).FastForward(context.Background(), sha('a')); !errors.Is(err, boom) {
		t.Errorf("FastForward = %v, want boom", err)
	}
}
