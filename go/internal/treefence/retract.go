package treefence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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

var (
	lstatKept    = os.Lstat
	readlinkKept = os.Readlink
	readKept     = os.ReadFile
)

func keepWrite(worktree, keepDir, rel string) error {
	src := filepath.Join(worktree, filepath.FromSlash(rel))
	info, err := lstatKept(src)
	if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("treefence: stat %s to keep: %w", rel, err)
	}
	dst := filepath.Join(keepDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("treefence: keep %s: %w", rel, err)
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return keepLink(src, dst, rel)
	case info.Mode().IsRegular():
		return keepFile(src, dst, rel, info.Mode().Perm())
	default:
		return nil
	}
}

func keepLink(src, dst, rel string) error {
	target, err := readlinkKept(src)
	if err != nil {
		return fmt.Errorf("treefence: read link %s to keep: %w", rel, err)
	}
	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("treefence: keep link %s: %w", rel, err)
	}
	return nil
}

func keepFile(src, dst, rel string, perm os.FileMode) error {
	data, err := readKept(src)
	if err != nil {
		return fmt.Errorf("treefence: read %s to keep: %w", rel, err)
	}
	if err := os.WriteFile(dst, data, perm); err != nil {
		return fmt.Errorf("treefence: keep %s: %w", rel, err)
	}
	return nil
}
