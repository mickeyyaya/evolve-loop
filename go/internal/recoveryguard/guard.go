// Package recoveryguard fences the recovery agent (ADR-0106): it may write the deliverables it was asked to
// repair and nothing else. Begin records every file in the run's workspace and fences the change's worktree
// before the agent runs; End puts back whatever the agent changed outside its grant and reports it. The kernel
// proves the grant, never the agent's own account of what it did.
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

// Scope is what one recovery dispatch may and may not touch.
type Scope struct {
	// Worktree is the change's checkout, fenced whole; "" when the phase had none.
	Worktree string
	// Workspace is the run directory; every entry in it is fenced unless allowed or unfenced.
	Workspace string
	// Allowed are the absolute paths of files the agent may create, change or remove: the violated
	// deliverables and its own report. A path that exists and is not a plain file is refused.
	Allowed []string
	// Unfenced are absolute paths of the telemetry the dispatch itself appends (signal and call logs), or
	// directories, matched exactly or below a directory; a name that merely begins the same is fenced.
	Unfenced []string
	// UnfencedStems are basename prefixes of the artifacts the dispatch creates directly under the workspace
	// with generated names (a sandbox profile dir, the agent's own logs). New entries with such a name survive
	// the fence and are reported in Outcome.Unfenced, never silently.
	UnfencedStems []string
}

// Guard is one fenced recovery dispatch.
type Guard struct {
	scope Scope
	fence *treefence.Fence
	// files holds every fenced regular file at Begin; other holds the type of every other fenced entry (a
	// link, a socket), which stays only as long as its type does and is never recreated; stemmed holds the
	// stem-tolerated entries already present at Begin.
	files   map[string]fileState
	other   map[string]fs.FileMode
	stemmed map[string]bool
}

type fileState struct {
	data []byte
	mode fs.FileMode
}

// Outcome is what End found and restored.
type Outcome struct {
	// Restored lists every workspace entry and worktree path put back, as absolute paths, sorted. A non-empty
	// list is an integrity violation: the agent left its grant.
	Restored []string
	// Unfenced lists the new stem-tolerated entries the dispatch left, sorted, for the caller to log.
	Unfenced []string
	// Err joins every failure to put a path back; the caller must treat the rung as failed.
	Err error
}

// Clean reports the agent stayed inside its grant and nothing had to be put back.
func (o Outcome) Clean() bool {
	return len(o.Restored) == 0 && o.Err == nil
}

// Begin snapshots the workspace and fences the worktree. It fails closed: a worktree that cannot be fenced, a
// workspace that cannot be read or an allowed path that is not a plain file (a link could point outside every
// fence) refuses the guard, so the rung does not run.
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

// walkWorkspace visits every fenced or stem-tolerated entry under the workspace without following links.
// Directories are descended, except a stem-tolerated one and one standing where a fenced file was, which are
// visited as entries.
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

// stem reports a direct child of the workspace whose name begins with a tolerated stem.
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

// End restores every fenced workspace entry the agent changed, planted, removed or swapped for something that
// is not a file, undoes an allowed path swapped for one, then restores the worktree, and reports it all.
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
