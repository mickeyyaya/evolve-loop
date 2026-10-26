// Package agentcmd holds `evolve agent`: one phase agent from the command line, with no cycle.
// See docs/architecture/packages/internal-cli-agentcmd.md.
package agentcmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	exitFail     = 1
	exitRefused  = 3
	exitDispatch = 21
	exitUsage    = 10
	exitInput    = 11
)

const usage = `Usage: evolve agent <subcommand>

  evolve agent run <profile> [--input FILE|-] [--file PATH]... [--json] [--keep] [--project-root P]
      Run one read-only phase agent with no cycle. The input is stdin, or --input FILE.
      Each --file adds one fenced section. stdout carries the report; --json carries
      one envelope (report, verdict, model, cost, tokens).
      Exit 0 PASS/WARN/SKIPPED, 1 FAIL, 3 refused, 21 dispatch error, 10 usage, 11 input.
  evolve agent list [--json] [--project-root P]
      Name each profile that evolve agent run accepts.
`

type Deps struct {
	InstallRouter func(projectRoot string) error
	Bridge        func(projectRoot string) core.Bridge
	Getenv        func(string) string
	Now           func() time.Time
}

type command struct{ deps Deps }

func New(d Deps) func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if d.InstallRouter == nil {
		d.InstallRouter = func(string) error { return nil }
	}
	if d.Bridge == nil {
		d.Bridge = phasecmd.DefaultBridge
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
