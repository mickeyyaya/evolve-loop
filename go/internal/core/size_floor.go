package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

func sizeFloorFailures(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	base := floorBase(in)
	prefix, err := moduleRelPrefix(in.Worktree, codequality.ModuleDir(in.Worktree))
	var refusal string
	if err == nil {
		paths := changedWorktreePathsSince(ctx, in.Worktree, base, "--no-renames")
		refusal, err = sizeratchet.CheckAgainstBase(prefix, paths, commentaudit.ReadAtBase(worktreeGit(ctx, in.Worktree), base), worktreeFile(in.Worktree))
	}
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN size floor could not judge the change against %s (%v); the repo-contract pack still runs the ratchet\n", base, err)
		return nil
	}
	if refusal == "" {
		return nil
	}
	return []string{fmt.Sprintf("size ratchet against the base %s: a listed function's ceiling is its size at the base, never its slack. Shrink the function back to its base size; do not edit %s:\n  %s", base, sizeratchet.OffendersRelPath, refusal)}
}

func moduleRelPrefix(worktree, moduleDir string) (string, error) {
	rel, err := filepath.Rel(worktree, moduleDir)
	if err != nil {
		return "", fmt.Errorf("module dir %s is not under the worktree %s: %w", moduleDir, worktree, err)
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel) + "/", nil
}
