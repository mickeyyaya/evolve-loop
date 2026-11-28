package inboxcmd

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	exitOK      = 0
	exitFailed  = 1
	exitIO      = 2
	exitRefused = 3
	exitUsage   = 10
)

type Deps struct {
	QueueGate          func(root, verb string, args []string, asJSON bool, stdout, stderr io.Writer) (int, bool)
	LaneForbidden      func(projectRoot string, warn io.Writer) func(string) bool
	PrintVerbJSON      func(verb string, v any, stdout, stderr io.Writer) int
	HumanWriter        func(stdout io.Writer, asJSON bool) io.Writer
	CheckRootHasEvolve func(op, root string, stderr io.Writer) int
	NewSignalCenter    func(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func())
}

func (d Deps) queueGate(root, verb string, args []string, asJSON bool, stdout, stderr io.Writer) (int, bool) {
	if d.QueueGate != nil {
		return d.QueueGate(root, verb, args, asJSON, stdout, stderr)
	}
	return exitOK, false
}

func (d Deps) laneForbidden(projectRoot string, warn io.Writer) func(string) bool {
	if d.LaneForbidden != nil {
		return d.LaneForbidden(projectRoot, warn)
	}
	return func(string) bool { return false }
}

func (d Deps) printVerbJSON(verb string, v any, stdout, stderr io.Writer) int {
	if d.PrintVerbJSON != nil {
		return d.PrintVerbJSON(verb, v, stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve %s: PrintVerbJSON dependency is not wired\n", verb)
	return exitIO
}

func (d Deps) humanWriter(stdout io.Writer, asJSON bool) io.Writer {
	if d.HumanWriter != nil {
		return d.HumanWriter(stdout, asJSON)
	}
	if asJSON {
		return io.Discard
	}
	return stdout
}

func (d Deps) checkRootHasEvolve(op, root string, stderr io.Writer) int {
	if d.CheckRootHasEvolve != nil {
		return d.CheckRootHasEvolve(op, root, stderr)
	}
	return exitOK
}

func (d Deps) newSignalCenter(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	if d.NewSignalCenter != nil {
		return d.NewSignalCenter(role, projectRoot, evolveDir, console)
	}
	return nil, func() {}
}
