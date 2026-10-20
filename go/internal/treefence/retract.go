package treefence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (s Snapshot) Retract(ctx context.Context, keepDir string) (Result, error) {
	if s.mode != allFiles {
		return Result{}, errNotRestorable
	}
	after, err := writeTree(ctx, s.Worktree)
	if err != nil || after == s.Tree {
		return Result{}, err
	}
	added, changed, err := s.diff(ctx, after)
	if err != nil {
		return Result{}, err
	}
	for _, rel := range append(added, changed...) {
		if err := keepWrite(s.Worktree, keepDir, rel); err != nil {
			return Result{}, err
		}
	}
	return s.Restore(ctx)
}

func keepWrite(worktree, keepDir, rel string) error {
	data, err := os.ReadFile(filepath.Join(worktree, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("treefence: read %s to keep: %w", rel, err)
	}
	dst := filepath.Join(keepDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("treefence: keep %s: %w", rel, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("treefence: keep %s: %w", rel, err)
	}
	return nil
}
