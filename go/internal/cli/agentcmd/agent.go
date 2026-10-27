// Package agentcmd holds `evolve agent`: one phase agent from the command line, with no cycle.
// See docs/architecture/packages/internal-cli-agentcmd.md.
package agentcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/treestate"
)

const (
	exitFail     = 1
	exitRefused  = 2
	exitDispatch = 3
	exitUsage    = 10
	exitInput    = 11
)

const usage = `Usage: evolve agent <subcommand>

  evolve agent run <profile> [--input FILE|-] [--file PATH]... [--worktree DIR [--writable PATH]...] [--json] [--keep] [--project-root P]
      Run one phase agent with no cycle, through the bridge. The input is stdin, or --input FILE.
      Each --file adds one fenced section. stdout carries the report.
      --json carries one envelope (report, verdict, driver, family, model, cost, tokens, launch record).
      Each run writes its launch record to .evolve/agent-runs/<run label>.json.
      evolve commit-gate run --reviewers run:<run label> attests it.
      --worktree runs the agent in a linked git worktree.
      A writer needs --worktree and one --writable PATH for each file that it can change.
      The pipeline restores each other file of the worktree after the run.
      Exit 0 PASS/WARN/SKIPPED, 1 FAIL, 2 refused, 3 dispatch error, 10 usage, 11 input.
  evolve agent list [--json] [--project-root P]
      Name each profile that evolve agent run accepts.
`

type Deps struct {
	InstallRouter func(projectRoot string) error
	Bridge        func(projectRoot string, signals *signalcenter.Center) core.Bridge
	Signals       func(projectRoot string, stderr io.Writer) (*signalcenter.Center, func())
	TreeState     func(ctx context.Context, dir string) (string, error)
	Getenv        func(string) string
	Now           func() time.Time
}

func defaultBridge(projectRoot string, _ *signalcenter.Center) core.Bridge {
	return phasecmd.DefaultBridge(projectRoot)
}

func noSignals(string, io.Writer) (*signalcenter.Center, func()) { return nil, func() {} }

func defaultTreeState(ctx context.Context, dir string) (string, error) {
	return treestate.SHA(ctx, sysexec.DefaultRunner, dir, nil)
}

type command struct{ deps Deps }

func New(d Deps) func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if d.InstallRouter == nil {
		d.InstallRouter = func(string) error { return nil }
	}
	if d.Bridge == nil {
		d.Bridge = defaultBridge
	}
	if d.Signals == nil {
		d.Signals = noSignals
	}
	if d.TreeState == nil {
		d.TreeState = defaultTreeState
	}
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return command{deps: d}.dispatch
}

func (c command) dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "run":
		return c.run(args[1:], stdin, stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "evolve agent: unknown subcommand %q\n\n%s", args[0], usage)
	return exitUsage
}
