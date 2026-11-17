package treestate

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func baseFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	g := gittest.Fixture(t)
	g.Write("a.txt", "one\ntwo\nthree\n")
	g.Git("add", ".")
	g.Git("commit", "-q", "-m", "base")
	return g
}

func TestChangeDigest_WorktreeChangeEqualsCommitted(t *testing.T) {
	t.Parallel()
	g := baseFixture(t)
	parent := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	g.Write("a.txt", "one\nTWO\nthree\n")
	g.Write("b.txt", "new\n")
	g.Git("add", ".")
	staged, err := SHA(context.Background(), sysexec.DefaultRunner, g.Dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.Git("commit", "-q", "-m", "change")
	commit := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	got, err := Between(context.Background(), sysexec.DefaultRunner, g.Dir, nil, parent, commit)
	if err != nil {
		t.Fatal(err)
	}
	if got != staged {
		t.Errorf("Between(parent, commit) = %q, want the staged digest %q", got, staged)
	}
}

func TestChangeDigest_AnUnstagedEditAtGateTimeDoesNotMatch(t *testing.T) {
	t.Parallel()
	g := baseFixture(t)
	parent := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	g.Write("a.txt", "one\nTWO\nthree\n")
	g.Git("add", ".")
	g.Write("a.txt", "one\nTWO\nthree\nextra\n")
	atGate, err := SHA(context.Background(), sysexec.DefaultRunner, g.Dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.Git("commit", "-q", "-m", "change")
	commit := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	got, err := Between(context.Background(), sysexec.DefaultRunner, g.Dir, nil, parent, commit)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || got == atGate {
		t.Errorf("Between = %q must differ from the gate digest %q that held an unstaged edit", got, atGate)
	}
}

func TestBetween_ReportsAFailedRun(t *testing.T) {
	t.Parallel()
	var args []string
	_, err := Between(context.Background(), fakeRunner("", 128, nil, &args, nil), "", nil, "p", "c")
	if err == nil {
		t.Fatal("Between accepted a fatal git exit")
	}
	if got := strings.Join(args, " "); got != "git diff p c" {
		t.Errorf("args = %q, want git diff p c", got)
	}
}

func TestBetween_AFailedRunNamesTheGitMessage(t *testing.T) {
	t.Parallel()
	run := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, "fatal: bad revision 'c'\n")
		return 128, nil
	}
	_, err := Between(context.Background(), run, "", nil, "p", "c")
	if err == nil || !strings.Contains(err.Error(), "fatal: bad revision 'c'") {
		t.Errorf("err = %v, want the git message", err)
	}
	if err != nil && !strings.HasPrefix(err.Error(), "git diff p c exit 128") {
		t.Errorf("err = %v, want it to name the revisions p c", err)
	}
}

func TestSHA_AFailedRunNamesHEAD(t *testing.T) {
	t.Parallel()
	_, err := SHA(context.Background(), fakeRunner("", 128, nil, new([]string), nil), "", nil)
	if err == nil || err.Error() != "git diff HEAD exit 128" {
		t.Errorf("err = %v, want git diff HEAD exit 128", err)
	}
}
