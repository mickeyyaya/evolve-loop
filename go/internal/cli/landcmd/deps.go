package landcmd

import (
	"errors"
	"fmt"
	"io"

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

func (d Deps) resolveRoot(projectRoot string, stderr io.Writer) (string, error) {
	if fn := d.ResolveRoot; fn != nil {
		return fn(projectRoot, stderr)
	}
	return "", errors.New("ResolveRoot dependency is not wired")
}

func (d Deps) printVerbJSON(verb string, v any, stdout, stderr io.Writer) int {
	if fn := d.PrintVerbJSON; fn != nil {
		return fn(verb, v, stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve %s: PrintVerbJSON dependency is not wired\n", verb)
	return exitIO
}

func (d Deps) newSignalCenter(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	if fn := d.NewSignalCenter; fn != nil {
		return fn(role, projectRoot, evolveDir, console)
	}
	return nil, func() {}
}

func (d Deps) refreshWorktree(root string) derived.Worktree {
	if fn := d.RefreshWorktree; fn != nil {
		return fn(root)
	}
	return core.DerivedWorktree(root)
}
