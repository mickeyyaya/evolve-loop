package explanationdocs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
)

func archiveDateDir() string {
	return filepath.Join("docs", "private", "research", "archived-"+time.Now().UTC().Format("2006-01-02"))
}

func ArchiveSupersededPredicatePackages(ctx context.Context, worktree, baseSHA string, cycle int) ([]string, error) {
	paths, err := changedSince(ctx, worktree, baseSHA)
	if err != nil {
		return nil, err
	}
	var archived, stagePaths []string
	for _, dir := range acssuite.AncestorCyclePackages(paths, cycle) {
		exists, err := existsAtBase(ctx, worktree, baseSHA, dir)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		rel, err := archivePredicatePackage(worktree, dir)
		if err != nil {
			return nil, err
		}
		archived = append(archived, rel)
		stagePaths = append(stagePaths, dir, rel)
	}
	if len(archived) == 0 {
		return nil, nil
	}
	args := append([]string{"-C", worktree, "add", "-A", "--"}, stagePaths...)
	if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("stage superseded predicate archive: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return archived, nil
}

func archivePredicatePackage(worktree, dir string) (string, error) {
	src := filepath.Join(worktree, filepath.FromSlash(dir))
	if err := requireRealDirectory(src, "superseded predicate package "+dir); err != nil {
		return "", err
	}
	archiveDir := filepath.ToSlash(filepath.Join(archiveDateDir(), "superseded-predicate-packages"))
	archiveAbs, err := ensureRealSubdirectories(worktree, archiveDir)
	if err != nil {
		return "", err
	}
	rel := filepath.ToSlash(filepath.Join(archiveDir, filepath.Base(dir)))
	dest := filepath.Join(archiveAbs, filepath.Base(dir))
	if _, err := os.Lstat(dest); err == nil {
		return "", fmt.Errorf("archive destination %s already exists", rel)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect archive destination %s: %w", rel, err)
	}
	if err := os.Rename(src, dest); err != nil {
		return "", fmt.Errorf("archive superseded predicate package %s: %w", dir, err)
	}
	return rel, nil
}
