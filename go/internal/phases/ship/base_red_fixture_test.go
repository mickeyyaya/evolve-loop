package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	redPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/dupfix"
	redTest     = "TestCount"
	redTestFile = "go/internal/dupfix/dup_test.go"
	redBody     = "package dupfix\n\nimport \"testing\"\n\nfunc TestCount(t *testing.T) { t.Fatal(\"count is above baseline\") }\n"
	greenBody   = "package dupfix\n\nimport \"testing\"\n\nfunc TestCount(t *testing.T) {}\n"
)

type baseRedLane struct {
	repo, wt, base string
	signals        *signalcenter.Center
	events         []signalcenter.Event
}

func newBaseRedLane(t *testing.T) *baseRedLane {
	t.Helper()
	repo := repoDeclaring(t, evolveLoopGoMod)
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(redTestFile)), redBody)
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "a red test lands on main")
	return (&baseRedLane{repo: repo}).laneOn(t, "cycle-1-pack")
}

func (l *baseRedLane) laneOn(t *testing.T, branch string) *baseRedLane {
	t.Helper()
	wt := makeWorktree(t, l.repo, branch)
	mustWrite(t, filepath.Join(wt, "go", "internal", "lane", "lane.go"), "package lane\n\nconst Lane = 1\n")
	runGit(t, wt, "add", "-A")
	mustWrite(t, filepath.Join(l.repo, ".evolve", "cycle-state.json"), `{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	lane := &baseRedLane{repo: l.repo, wt: wt, base: strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "main")), signals: signalcenter.New()}
	lane.signals.Subscribe(func(e signalcenter.Event) { lane.events = append(lane.events, e) })
	return lane
}

func (l *baseRedLane) landOnMain(t *testing.T, msg string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		mustWrite(t, filepath.Join(l.repo, filepath.FromSlash(rel)), body)
	}
	runGit(t, l.repo, "add", "-A")
	runGit(t, l.repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", msg)
}

func (l *baseRedLane) ship(t *testing.T) error {
	t.Helper()
	p := New(Config{Runner: execRunner, RepoContractGate: "enforce", Signals: l.signals})
	_, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: l.repo, Workspace: filepath.Join(l.repo, ".evolve", "runs", "cycle-1"),
		Worktree: l.wt, WorktreeBaseSHA: l.base, Context: map[string]string{"commit_message": "evolve-cycle 1: goal=test"},
	})
	return err
}

func (l *baseRedLane) emitted(code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range l.events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

type scriptedBase struct {
	calls   int
	dirs    []string
	answers [][]baseattr.Key
	err     error
}

func (s *scriptedBase) run(_ context.Context, dir string, _ baseattr.Reds) ([]baseattr.Key, error) {
	s.calls++
	s.dirs = append(s.dirs, dir)
	if s.err != nil {
		return nil, s.err
	}
	return s.answers[min(s.calls, len(s.answers))-1], nil
}

func (l *baseRedLane) options(run baseattr.Runner) *Options {
	return &Options{
		Class: ClassCycle, ProjectRoot: l.repo, ActiveWorktree: l.wt, WorktreeBaseSHA: l.base,
		CycleID: 1, RunID: "test-run", Runner: execRunner, Signals: l.signals, baseRun: run,
	}
}

var redKey = baseattr.Key{Package: redPkg, Test: redTest}

func (l *baseRedLane) gateOnly(t *testing.T) error {
	t.Helper()
	p := New(Config{Runner: execRunner, RepoContractGate: "enforce", Signals: l.signals})
	req := core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: l.repo, Workspace: filepath.Join(l.repo, ".evolve", "runs", "cycle-1"),
		Worktree: l.wt, WorktreeBaseSHA: l.base,
	}
	opts := p.shipOptions(req, "evolve-cycle 1")
	return p.runRepoContractGate(context.Background(), req, &opts)
}
