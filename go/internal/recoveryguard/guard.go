// Package recoveryguard fences the recovery agent: it may write
// the deliverables it was asked to repair and nothing else.
package recoveryguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type Scope struct {
	Worktree      string
	Workspace     string
	Allowed       []string
	Unfenced      []string
	UnfencedStems []string
}

type Guard struct {
	scope   Scope
	fence   *treefence.Fence
	files   map[string]fileState
	other   map[string]fs.FileMode
	stemmed map[string]bool
}

type fileState struct {
	data []byte
	mode fs.FileMode
}

type Outcome struct {
	Restored []string
	Unfenced []string
	Err      error
}

func (o Outcome) Clean() bool {
	return len(o.Restored) == 0 && o.Err == nil
}

func Begin(ctx context.Context, scope Scope) (*Guard, error) {
	if scope.Workspace == "" {
		return nil, errors.New("recoveryguard: a workspace is required")
	}
	for _, a := range scope.Allowed {
		if info, err := os.Lstat(a); err == nil && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("recoveryguard: allowed path %s is not a plain file", a)
		}
	}
	g := &Guard{scope: scope, files: map[string]fileState{}, other: map[string]fs.FileMode{}, stemmed: map[string]bool{}}
	if scope.Worktree != "" {
		g.fence = treefence.Begin(ctx, scope.Worktree, true)
		if err := g.fence.TakeErr(); err != nil {
			return nil, fmt.Errorf("recoveryguard: fence the worktree: %w", err)
		}
	}
	err := g.walkWorkspace(func(path string, d fs.DirEntry) error {
		if g.stem(path) {
			g.stemmed[path] = true
			return nil
		}
		if !d.Type().IsRegular() {
			g.other[path] = d.Type()
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("recoveryguard: stat %s: %w", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("recoveryguard: snapshot %s: %w", path, err)
		}
		g.files[path] = fileState{data: data, mode: info.Mode().Perm()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Guard) walkWorkspace(visit func(path string, d fs.DirEntry) error) error {
	return filepath.WalkDir(g.scope.Workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("recoveryguard: walk %s: %w", path, err)
		}
		if path == g.scope.Workspace {
			return nil
		}
		if g.allowed(path) || g.unfenced(path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		_, fenced := g.files[path]
		if d.IsDir() && !fenced && !g.stem(path) {
			return nil
		}
		if err := visit(path, d); err != nil {
			return err
		}
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
}

func (g *Guard) allowed(path string) bool {
	return slices.Contains(g.scope.Allowed, path)
}

func (g *Guard) unfenced(path string) bool {
	for _, p := range g.scope.Unfenced {
		if path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (g *Guard) stem(path string) bool {
	if filepath.Dir(path) != g.scope.Workspace {
		return false
	}
	base := filepath.Base(path)
	for _, s := range g.scope.UnfencedStems {
		if strings.HasPrefix(base, s) {
			return true
		}
	}
	return false
}

func (g *Guard) End(ctx context.Context) Outcome {
	var out Outcome
	var errs []error
	seen := map[string]bool{}
	err := g.walkWorkspace(func(path string, d fs.DirEntry) error {
		if g.stem(path) {
			if !g.stemmed[path] {
				out.Unfenced = append(out.Unfenced, path)
			}
			return nil
		}
		seen[path] = true
		want, fenced := g.files[path]
		switch {
		case !d.Type().IsRegular():
			if !fenced && g.other[path] == d.Type() {
				return nil
			}
			out.Restored = append(out.Restored, path)
			if err := os.RemoveAll(path); err != nil {
				errs = append(errs, fmt.Errorf("recoveryguard: remove %s: %w", path, err))
			} else if fenced {
				errs = append(errs, writeBack(path, want))
			}
		case !fenced:
			out.Restored = append(out.Restored, path)
			if err := os.Remove(path); err != nil {
				errs = append(errs, fmt.Errorf("recoveryguard: remove %s: %w", path, err))
			}
		default:
			got, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("recoveryguard: read %s: %w", path, err)
			}
			if !bytes.Equal(got, want.data) {
				out.Restored = append(out.Restored, path)
				errs = append(errs, writeBack(path, want))
			}
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	for _, path := range slices.Sorted(maps.Keys(g.files)) {
		if !seen[path] {
			out.Restored = append(out.Restored, path)
			errs = append(errs, writeBack(path, g.files[path]))
		}
	}
	for _, a := range g.scope.Allowed {
		if info, err := os.Lstat(a); err == nil && !info.Mode().IsRegular() {
			out.Restored = append(out.Restored, a)
			if err := os.RemoveAll(a); err != nil {
				errs = append(errs, fmt.Errorf("recoveryguard: remove %s: %w", a, err))
			}
		}
	}
	if g.fence != nil {
		fo := g.fence.End(ctx)
		for _, rel := range fo.Restored {
			out.Restored = append(out.Restored, filepath.Join(g.scope.Worktree, filepath.FromSlash(rel)))
		}
		if fo.RestoreErr != nil {
			errs = append(errs, fmt.Errorf("recoveryguard: restore the worktree: %w", fo.RestoreErr))
		}
	}
	slices.Sort(out.Restored)
	slices.Sort(out.Unfenced)
	out.Err = errors.Join(errs...)
	return out
}

func writeBack(path string, want fileState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recoveryguard: restore %s: %w", path, err)
	}
	if err := os.WriteFile(path, want.data, want.mode); err != nil {
		return fmt.Errorf("recoveryguard: restore %s: %w", path, err)
	}
	return nil
}
