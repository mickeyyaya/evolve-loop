package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type derivedRegen struct {
	Repo     integration.Repo
	Dir      string
	Worktree func(worktree string) derived.Worktree
}

func (r derivedRegen) Regen(ctx context.Context, onto, conflictedTree string, paths []string) (string, error) {
	entries, err := regenEntries(paths)
	if err != nil {
		return "", err
	}
	dir, err := r.checkout(ctx, onto)
	if err != nil {
		return "", err
	}
	defer r.release(ctx, dir)
	return r.regenerateIn(ctx, dir, conflictedTree, entries)
}

func regenEntries(paths []string) ([]derived.Entry, error) {
	var entries []derived.Entry
	for _, p := range paths {
		e, _, ok := derived.OutputOf(p)
		if !ok {
			return nil, fmt.Errorf("integration: %s is not a derived output", p)
		}
		if !slices.ContainsFunc(entries, func(have derived.Entry) bool { return have.Name == e.Name }) {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (r derivedRegen) checkout(ctx context.Context, onto string) (string, error) {
	if _, err := r.Repo.Git(ctx, r.Repo.Dir, "", "worktree", "prune"); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(r.Dir, "regen-")
	if err != nil {
		return "", err
	}
	if _, err := r.Repo.Git(ctx, r.Repo.Dir, "", "worktree", "add", "--detach", dir, onto); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func (r derivedRegen) release(ctx context.Context, dir string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), regenCleanup)
	defer cancel()
	_, _ = r.Repo.Git(cleanup, r.Repo.Dir, "", "worktree", "remove", "--force", dir)
	_ = os.RemoveAll(dir)
}

func (r derivedRegen) regenerateIn(ctx context.Context, dir, conflictedTree string, entries []derived.Entry) (string, error) {
	if _, err := r.Repo.Git(ctx, dir, "", "read-tree", "-u", "--reset", conflictedTree); err != nil {
		return "", err
	}
	worktree := r.Worktree(dir)
	add := []string{"add", "-A", "--"}
	for _, e := range entries {
		if err := worktree.Regenerate(ctx, e); err != nil {
			return "", err
		}
		add = append(add, e.Pathspecs()...)
	}
	if _, err := r.Repo.Git(ctx, dir, "", add...); err != nil {
		return "", err
	}
	out, err := r.Repo.Git(ctx, dir, "", "write-tree")
	return strings.TrimSpace(out), err
}

const (
	regenDir     = "integration-regen"
	regenCleanup = 30 * time.Second
)

func (e integrationEnv) regenPort(repo integration.Repo, projectRoot string, warn io.Writer) integration.RegenFunc {
	if e.Derived == nil {
		return nil
	}
	return e.regen(repo, projectRoot, warn).Regen
}

func (e integrationEnv) regen(repo integration.Repo, projectRoot string, warn io.Writer) derivedRegen {
	dir := filepath.Join(paths.EvolveDirOf(projectRoot), "ephemeral", regenDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(warn, "%screate %s: %v\n", integrationPrefix, dir, err)
	}
	return derivedRegen{Repo: repo, Dir: dir, Worktree: e.Derived}
}
