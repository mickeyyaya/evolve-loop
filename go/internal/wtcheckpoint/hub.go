package wtcheckpoint

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

type Hub struct{ plane.Hub }

func ResolveHub(projectRoot string) (Hub, error) {
	h, err := plane.ResolveHub(projectRoot)
	if err != nil {
		return Hub{}, fmt.Errorf("wtcheckpoint: %w", err)
	}
	return Hub{h}, nil
}

func (h Hub) git() gitexec.Git { return gitexec.Isolated(h.Store) }

type listedWorktree struct {
	path     string
	prunable bool
}

func (h Hub) linked(ctx context.Context) ([]listedWorktree, error) {
	out, err := h.git().Output(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var all []listedWorktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			all = append(all, listedWorktree{path: strings.TrimPrefix(line, "worktree ")})
		case strings.HasPrefix(line, "prunable") && len(all) > 0:
			all[len(all)-1].prunable = true
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all[1:], nil
}

func (h Hub) Worktree(ctx context.Context, dir string) (Worktree, error) {
	linked, err := h.linked(ctx)
	if err != nil {
		return Worktree{}, err
	}
	want := realPath(dir)
	for _, l := range linked {
		if !l.prunable && realPath(l.path) == want {
			return worktreeAt(l.path), nil
		}
	}
	return Worktree{}, fmt.Errorf("wtcheckpoint: refused: %s is not a linked worktree of %s", dir, h.Store)
}

func (h Hub) DevWorktrees(ctx context.Context) ([]Worktree, error) {
	linked, err := h.linked(ctx)
	if err != nil {
		return nil, err
	}
	dev := realPath(h.DevDir()) + string(filepath.Separator)
	var out []Worktree
	for _, l := range linked {
		if l.prunable || !strings.HasPrefix(realPath(l.path), dev) {
			continue
		}
		out = append(out, worktreeAt(l.path))
	}
	return out, nil
}

func worktreeAt(dir string) Worktree {
	return Worktree{Dir: dir, Name: filepath.Base(dir)}
}

func realPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
