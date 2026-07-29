// Package plane classifies which worktree plane a project root occupies
// (ADR-0080 runtime/console separation). The loop runs in a dedicated LINKED
// worktree; the PRIMARY checkout is the operator's console. Batch-15 lost
// four lanes (1149-1152) to operator activity in a shared checkout — the
// boot-time classification below is the standing tripwire against relaunching
// the loop into the operator's tree.
//
// Filesystem-only by design: `.git` is a directory in the primary checkout
// and a `gitdir:` pointer file in a linked worktree, and HEAD is a one-line
// ref file in both. No git subprocess — hermetic, dependency-free, callable
// before any bridge or exec seam exists at boot.
package plane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Info is one project root's plane classification.
type Info struct {
	// IsLinkedWorktree is true when .git is a gitdir pointer file — the
	// ADR-0080 runtime plane shape. False = the primary checkout (console).
	IsLinkedWorktree bool
	// Branch is the checked-out branch name; empty on a detached HEAD.
	Branch string
	// GitDir is the resolved git directory (the hub's worktrees/<name> dir
	// for a linked worktree, <root>/.git for the primary).
	GitDir string
}

// Classify reads the plane shape of projectRoot from the filesystem.
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
		// The prefix is REQUIRED (review MEDIUM: TrimPrefix made any non-empty
		// .git file read as a linked worktree — the tripwire failing OPEN in
		// the primary checkout, the one direction that matters).
		rest, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
		gitdir := strings.TrimSpace(rest)
		if !ok || gitdir == "" {
			return Info{}, fmt.Errorf("plane: %s is neither a git dir nor a gitdir pointer", dotGit)
		}
		// A relative pointer (git >=2.48 worktree.useRelativePaths, submodule
		// checkouts) resolves against the CHECKOUT, not the process cwd —
		// unresolved it silently blanked Branch and turned the S3 sync into a
		// permanent no-op (review MEDIUM).
		if !filepath.IsAbs(gitdir) {
			gitdir = filepath.Clean(filepath.Join(projectRoot, gitdir))
		}
		info.IsLinkedWorktree = true
		info.GitDir = gitdir
	}
	info.Branch = headBranch(info.GitDir)
	return info, nil
}

// headBranch parses HEAD's "ref: refs/heads/<name>" line; a detached HEAD
// (bare SHA) or unreadable HEAD yields "" — absence of a branch is data, not
// an error, for classification purposes.
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

// ConsoleLeaseFileName is the operator console lease's file name inside the
// git common dir — the ONE string the lease writer (cli/opscmd) and reader
// (internal/core) must agree on (review MEDIUM: two hardcoded copies would
// let a rename silently disarm every lease).
const ConsoleLeaseFileName = "evolve-console-lease.json"

// CommonGitDir resolves the SHARED git directory (the hub) for a classified
// root: the primary's .git itself, or a linked worktree's gitdir walked
// through its `commondir` file. The hub sits OUTSIDE every worktree — the
// operator console lease lives there precisely so no lane phase can author
// or shadow it from inside a checkout (ADR-0080 S4 review BLOCK).
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

// BootLine renders the one-line plane report the loop prints at launch. A
// linked worktree (the runtime plane) is informational; the PRIMARY checkout
// carries the ADR-0080 warning — operator edits in a shared tree are exactly
// what killed lanes 1149-1152.
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
