// Package plane classifies which worktree plane a project root occupies,
// the linked runtime worktree or the primary console checkout.
package plane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	IsLinkedWorktree bool
	Branch           string
	GitDir           string
}

func Classify(projectRoot string) (Info, error) {
	dotGit := filepath.Join(projectRoot, ".git")
	fi, err := os.Stat(dotGit)
	if err != nil {
		return Info{}, fmt.Errorf("plane: %s is not a git checkout: %w", projectRoot, err)
	}
	info := Info{GitDir: dotGit}
	if !fi.IsDir() {
		raw, rerr := os.ReadFile(dotGit)
		if rerr != nil {
			return Info{}, fmt.Errorf("plane: read gitdir pointer: %w", rerr)
		}
		rest, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
		gitdir := strings.TrimSpace(rest)
		if !ok || gitdir == "" {
			return Info{}, fmt.Errorf("plane: %s is neither a git dir nor a gitdir pointer", dotGit)
		}
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Clean(filepath.Join(projectRoot, gitdir))
		}
		info.IsLinkedWorktree = true
		info.GitDir = gitdir
	}
	info.Branch = headBranch(info.GitDir)
	return info, nil
}

func headBranch(gitDir string) string {
	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(raw))
	const refPrefix = "ref: refs/heads/"
	if !strings.HasPrefix(head, refPrefix) {
		return ""
	}
	return strings.TrimPrefix(head, refPrefix)
}

const ConsoleLeaseFileName = "evolve-console-lease.json"

func CommonGitDir(i Info) (string, error) {
	if !i.IsLinkedWorktree {
		return i.GitDir, nil
	}
	raw, err := os.ReadFile(filepath.Join(i.GitDir, "commondir"))
	if err != nil {
		return "", fmt.Errorf("plane: linked worktree without commondir: %w", err)
	}
	common := strings.TrimSpace(string(raw))
	if !filepath.IsAbs(common) {
		common = filepath.Clean(filepath.Join(i.GitDir, common))
	}
	return common, nil
}

func BootLine(i Info) string {
	branch := i.Branch
	if branch == "" {
		branch = "(detached)"
	}
	if i.IsLinkedWorktree {
		return fmt.Sprintf("[loop] plane: linked worktree (runtime plane) branch=%s", branch)
	}
	return fmt.Sprintf("[loop] plane: PRIMARY checkout branch=%s — operator activity in this tree can kill lanes; launch the loop from a dedicated runtime worktree (ADR-0080)", branch)
}
