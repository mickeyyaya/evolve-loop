package landcmd

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	exitOK       = 0
	exitRefused  = 1
	exitIO       = 2
	exitBoundary = 3
	exitUsage    = 10
)

type Deps struct {
	ResolveRoot     func(projectRoot string, stderr io.Writer) (string, error)
	PrintVerbJSON   func(verb string, v any, stdout, stderr io.Writer) int
	NewSignalCenter func(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func())
	DarwinSet       func(args []string, stdout, stderr io.Writer) int
	RefreshWorktree func(root string) derived.Worktree
}

var current atomic.Pointer[Deps]

func host() Deps {
	if d := current.Load(); d != nil {
		return *d
	}
	return Deps{}
}

func resolveRoot(projectRoot string, stderr io.Writer) (string, error) {
	if fn := host().ResolveRoot; fn != nil {
		return fn(projectRoot, stderr)
	}
	return "", errors.New("ResolveRoot dependency is not wired")
}

func printVerbJSON(verb string, v any, stdout, stderr io.Writer) int {
	if fn := host().PrintVerbJSON; fn != nil {
		return fn(verb, v, stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve %s: PrintVerbJSON dependency is not wired\n", verb)
	return exitIO
}

func newSignalCenter(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	if fn := host().NewSignalCenter; fn != nil {
		return fn(role, projectRoot, evolveDir, console)
	}
	return nil, func() {}
}

func refreshWorktree(root string) derived.Worktree {
	if fn := host().RefreshWorktree; fn != nil {
		return fn(root)
	}
	return core.DerivedWorktree(root)
}
