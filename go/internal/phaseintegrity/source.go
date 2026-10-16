// Package phaseintegrity is the production digest source of the per-phase integrity chain.
// See docs/architecture/packages/internal-phaseintegrity.md.
package phaseintegrity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/selfsha"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

const gitTreeTimeout = 30 * time.Second

type Source struct {
	BinaryPath   string
	ProfilePath  string
	ReportPath   string
	WorktreePath string
	GitTree      func(worktree string) (string, error)
}

func (s Source) BinarySHA() (string, error) {
	if s.BinaryPath == "" {
		return selfsha.Running()
	}
	return selfsha.Of(s.BinaryPath)
}

func (s Source) BinaryCommit() string { return version.Commit() }

func (s Source) ProfileSHA() (string, error) { return shaOrEmpty(s.ProfilePath) }

func (s Source) ReportSHA() (string, error) { return shaOrEmpty(s.ReportPath) }

func (s Source) TreeSHA() (string, error) {
	if s.WorktreePath == "" {
		return "", nil
	}
	if s.GitTree != nil {
		return s.GitTree(s.WorktreePath)
	}
	return defaultGitTree(s.WorktreePath)
}

func shaOrEmpty(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("phaseintegrity: stat %s: %w", path, err)
	}
	return selfsha.Of(path)
}

func defaultGitTree(worktree string) (string, error) {
	if !filepath.IsAbs(worktree) {
		return "", fmt.Errorf("phaseintegrity: worktree path must be absolute, got %q", worktree)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTreeTimeout)
	defer cancel()
	out, err := sysexec.Command(ctx, "git", "-C", worktree, "write-tree").Output()
	if err != nil {
		return "", fmt.Errorf("phaseintegrity: git write-tree in %s: %w", worktree, err)
	}
	return strings.TrimSpace(string(out)), nil
}
