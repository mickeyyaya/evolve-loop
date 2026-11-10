package core

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const baseRedPkg = "example.com/m/q"

type baseRedFixture struct {
	in      ReviewInput
	signals *signalcenter.Center
	stderr  *injectedStderr
	runs    *runCounter
}

type runCounter struct {
	mu    sync.Mutex
	calls int
	red   []baseattr.Key
	err   error
}

func (c *runCounter) run(_ context.Context, _ string, r baseattr.Reds) ([]baseattr.Key, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	var out []baseattr.Key
	for _, k := range r.Keys {
		for _, red := range c.red {
			if k == red {
				out = append(out, k)
			}
		}
	}
	return out, nil
}

func newBaseRedFixture(t *testing.T, red ...string) baseRedFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	var keys []baseattr.Key
	for _, name := range red {
		keys = append(keys, baseattr.Key{Package: baseRedPkg, Test: name})
	}
	counter := &runCounter{red: keys}
	stderr, signals := &injectedStderr{}, signalcenter.New()
	in := ReviewInput{
		Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: repo.Dir, WorktreeBaseSHA: base,
		Stderr: stderr, Signals: signals, baseRun: counter.run,
	}
	return baseRedFixture{in: in, signals: signals, stderr: stderr, runs: counter}
}

func (f baseRedFixture) codes() []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range f.signals.Recent() {
		out = append(out, e.Code)
	}
	return out
}

func (f baseRedFixture) saw(code signalcenter.Code) bool {
	for _, c := range f.codes() {
		if c == code {
			return true
		}
	}
	return false
}

func packRedNames(tests ...string) []string {
	out := make([]string, len(tests))
	for i, t := range tests {
		out[i] = baseRedPkg + "." + t
	}
	return out
}

func goTestRed(tests ...string) string {
	var b strings.Builder
	for _, t := range tests {
		b.WriteString("--- FAIL: " + t + " (0.00s)\n")
	}
	b.WriteString("FAIL\t" + baseRedPkg + "\t0.1s\nFAIL\n")
	return b.String()
}
