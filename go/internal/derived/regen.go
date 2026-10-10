package derived

import (
	"context"
	"fmt"
	"strings"
)

type Generator func(ctx context.Context, evolveArgs ...string) error

type Git func(ctx context.Context, args ...string) (string, int, error)

type Worktree struct {
	Run  Generator
	Git  Git
	Base string
}

const (
	leftoverMarker    = "leftover conflict marker"
	diffCheckClean    = 0
	diffCheckProblems = 2
)

func (w Worktree) Refresh(ctx context.Context, e Entry) error {
	if w.Run(ctx, e.Check...) == nil {
		return nil
	}
	return w.Regenerate(ctx, e)
}

func (w Worktree) Regenerate(ctx context.Context, e Entry) error {
	if err := w.Run(ctx, e.Generate...); err != nil {
		return fmt.Errorf("regenerate %s via `evolve %s`: %w", e.Name, strings.Join(e.Generate, " "), err)
	}
	if err := w.Run(ctx, e.Check...); err != nil {
		return fmt.Errorf("check %s after regeneration via `evolve %s`: %w", e.Name, strings.Join(e.Check, " "), err)
	}
	return w.refuseLeftoverMarkers(ctx, e)
}

func (w Worktree) refuseLeftoverMarkers(ctx context.Context, e Entry) error {
	out, code, err := w.Git(ctx, append([]string{"diff", "--check", w.Base, "--"}, e.Pathspecs()...)...)
	if err != nil {
		return fmt.Errorf("diff --check %s against %s: %w", e.Name, w.Base, err)
	}
	if code != diffCheckClean && code != diffCheckProblems {
		return fmt.Errorf("diff --check %s against %s: exit %d: %s", e.Name, w.Base, code, strings.TrimSpace(out))
	}
	if strings.Contains(out, leftoverMarker) {
		return fmt.Errorf("%s keeps a %s after regeneration: %s", e.Name, leftoverMarker, strings.TrimSpace(out))
	}
	return nil
}
