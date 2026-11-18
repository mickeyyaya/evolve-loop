package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

type scriptedGitCall struct {
	stdout string
	exit   int
}

type exitingGit struct {
	script map[string]scriptedGitCall
}

func (g *exitingGit) run(_ context.Context, name, _ string, args []string, _ []string,
	_ io.Reader, stdout, _ io.Writer) (int, error) {
	if name != "git" || len(args) < 3 {
		return 0, nil
	}
	call := g.script[args[2]]
	_, _ = io.WriteString(stdout, call.stdout)
	return call.exit, nil
}

func detectorOverExitingGit(t *testing.T, g *exitingGit) *gitEvidenceDetector {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("tok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Runner: g.run}.withDefaults()
	return newGitEvidenceDetector(&Config{Workspace: ws, Worktree: ws, Agent: "scout"}, deps)
}

func evidenceCommitMessage() string {
	return "subject\n" + evidence.Trailer{Phase: "scout", Challenge: "tok", Cycle: 5}.Build()
}

func TestExitingGit_ReportsTheScriptedExitCodeAndTheStdout(t *testing.T) {
	t.Parallel()
	g := &exitingGit{script: map[string]scriptedGitCall{"rev-list": {stdout: "partial", exit: 1}}}
	var out strings.Builder
	code, err := g.run(context.Background(), "git", "", []string{"-C", "wt", "rev-list", "a..b"}, nil, nil, &out, io.Discard)
	if err != nil || code != 1 || out.String() != "partial" {
		t.Fatalf("fake git: code=%d err=%v out=%q, want code=1 err=nil out=partial", code, err, out.String())
	}
}

func TestGitEvidence_RevListExitOneWithPartialStdoutIsAnErrorNotEvidence(t *testing.T) {
	t.Parallel()
	g := &exitingGit{script: map[string]scriptedGitCall{
		"rev-parse": {stdout: "sha0"},
		"rev-list":  {stdout: "sha1", exit: 1},
		"log":       {stdout: evidenceCommitMessage()},
	}}
	d := detectorOverExitingGit(t, g)
	if ready, _, _, _ := d.poll(context.Background()); ready {
		t.Fatal("the first poll only baselines HEAD and must not complete")
	}
	g.script["rev-parse"] = scriptedGitCall{stdout: "sha1"}
	for i := 0; i < 3; i++ {
		ready, ev, note, err := d.poll(context.Background())
		if ready {
			t.Fatalf("poll %d: a rev-list that exited 1 completed on its partial stdout (sha=%q): a failed git command was read as data", i, ev.CommitSHA)
		}
		if note != "" || err != nil {
			t.Fatalf("poll %d: note=%q err=%v, want a quiet not-ready", i, note, err)
		}
	}
	if d.baseline != "sha0" {
		t.Errorf("baseline = %q, want sha0: a rev-list fault must not re-baseline past unseen commits", d.baseline)
	}
}

func TestGitEvidence_LogExitOneWithTheEvidenceMessageIsNotEvidence(t *testing.T) {
	t.Parallel()
	g := &exitingGit{script: map[string]scriptedGitCall{
		"rev-parse": {stdout: "sha0"},
		"rev-list":  {stdout: "sha1"},
		"log":       {stdout: evidenceCommitMessage(), exit: 128},
	}}
	d := detectorOverExitingGit(t, g)
	d.poll(context.Background())
	g.script["rev-parse"] = scriptedGitCall{stdout: "sha1"}
	if ready, ev, _, _ := d.poll(context.Background()); ready {
		t.Fatalf("a git log that exited 128 verified a commit (sha=%q): a failed git command was read as data", ev.CommitSHA)
	}
}

func TestGitEvidence_RevParseExitOneNeverBecomesTheBaseline(t *testing.T) {
	t.Parallel()
	g := &exitingGit{script: map[string]scriptedGitCall{"rev-parse": {stdout: "half-written-sha", exit: 128}}}
	d := detectorOverExitingGit(t, g)
	d.poll(context.Background())
	if d.haveBaseline || d.baseline != "" {
		t.Fatalf("baseline=%q have=%v: a rev-parse that exited 128 became the HEAD baseline", d.baseline, d.haveBaseline)
	}
}

func TestGitEvidence_GitCmdContract_NonZeroExitIsAnErrorForEverySubcommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sub  string
		args []string
	}{
		{"rev-parse", []string{"rev-parse", "HEAD"}},
		{"rev-list", []string{"rev-list", "sha0..HEAD"}},
		{"log", []string{"log", "-1", "--format=%B", "sha1"}},
	}
	for _, tc := range cases {
		for _, exit := range []int{1, 128, -1} {
			g := &exitingGit{script: map[string]scriptedGitCall{tc.sub: {stdout: "partial", exit: exit}}}
			d := detectorOverExitingGit(t, g)
			if out, err := d.gitCmd(context.Background(), tc.args...); err == nil {
				t.Errorf("git %s exit %d: err = nil, out = %q, want an error for a non-zero exit", tc.sub, exit, out)
			}
		}
	}
}

func TestGitEvidence_GitCmdContract_ZeroExitKeepsTheTrimmedStdout(t *testing.T) {
	t.Parallel()
	g := &exitingGit{script: map[string]scriptedGitCall{"rev-parse": {stdout: "sha0\n"}}}
	d := detectorOverExitingGit(t, g)
	out, err := d.gitCmd(context.Background(), "rev-parse", "HEAD")
	if err != nil || out != "sha0" {
		t.Fatalf("exit 0: out=%q err=%v, want sha0 and nil", out, err)
	}
}
