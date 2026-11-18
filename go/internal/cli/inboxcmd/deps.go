package inboxcmd

import (
	"io"
	"sync/atomic"

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

var current atomic.Pointer[Deps]

func host() Deps {
	if d := current.Load(); d != nil {
		return *d
	}
	return Deps{}
}
