package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var evolveDeliverablePrefixes = []string{
	".evolve/evals/",
	".evolve/phases/",
	".evolve/commit-prefix-scope.json",
}

// isEvolveDeliverablePath matches a directory prefix but requires exact
// equality for a file entry, so a look-alike name cannot false-positive.
func isEvolveDeliverablePath(p string) bool {
	for _, pfx := range evolveDeliverablePrefixes {
		if strings.HasSuffix(pfx, "/") {
			if p == strings.TrimSuffix(pfx, "/") || strings.HasPrefix(p, pfx) {
				return true
			}
		} else if p == pfx {
			return true
		}
	}
	return false
}

// recoverBuildLeak relocates or discards a phase's main-tree leak so its
// cycle can continue past the tree-diff guard. sourceWriter=false phases
// (triage/audit/scout/bug-reproduction) relocate only genuine untracked
// leaks, leaving evolve deliverables and tracked edits for the guard to judge.
func recoverBuildLeak(ctx context.Context, projectRoot, worktree string, baseline map[string]bool, sourceWriter bool) bool {
	if worktree == "" {
		return true // no worktree to relocate into → degrade (caller guards this anyway)
	}
	if inPlaceWorktree(worktree, projectRoot) {
		return true // the tree IS the worktree: nothing to relocate, nothing to check out (inPlaceWorktree)
	}
	// -uall lists untracked files individually, so os.Rename here never hits
	// a directory collision.
	out, code, err := gitCapture(ctx, projectRoot, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		// Can't determine leaks → degrade to the tree-diff guard (true); false
		// is reserved for a detected leak that couldn't be safely recovered.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git status failed (rc=%d): %v — degrading to tree-diff guard\n", code, err)
		return true
	}
	var relocated []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		p := porcelainPath(line)
		if p == "" || baseline[p] {
			continue
		}
		// Skip paths isLegitimateMainTreePath classifies as runtime state (the
		// same classification the boundary guard uses); evolveDeliverablePrefixes
		// are NOT skipped, since those relocate into the worktree like any other leak.
		if isLegitimateMainTreePath(p) && !buildArtifacts[p] {
			continue
		}
		xy := line[:2]
		switch {
		case strings.Contains(xy, "?"): // untracked file → relocate out of the main tree
			if !sourceWriter && isEvolveDeliverablePath(p) {
				continue
			}
			src := filepath.Join(projectRoot, p)
			dst := filepath.Join(worktree, p)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: mkdir for %s: %v\n", p, err)
				return false
			}
			if err := moveFile(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate %s: %v\n", p, err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked %s out of main tree\n", p)
			// Stage only for source writers, so the auditor's `git diff HEAD`
			// sees builder source; a non-source leak just needs to vacate main.
			if sourceWriter {
				relocated = append(relocated, p)
			}
		case buildArtifacts[p]: // rebuilt release binary leaked → always discard
			if err := discardMainLeak(ctx, projectRoot, p); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked rebuilt artifact %s\n", p)
		case strings.Contains(xy, "M"): // modified tracked file (exists at HEAD)
			if !sourceWriter {
				// A non-source-writer phase must not edit tracked source; leave
				// it for the tree-diff guard to abort.
				continue
			}
			if worktreeCleanForPath(ctx, worktree, p) {
				if err := relocateTrackedEdit(ctx, projectRoot, worktree, p); err != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate tracked edit %s: %v\n", p, err)
					return false
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked tracked edit %s into worktree\n", p)
				relocated = append(relocated, p)
			} else {
				if err := discardMainLeak(ctx, projectRoot, p); err != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
					return false
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked main-tree change %s (worktree diverged)\n", p)
			}
		case strings.ContainsAny(xy, "AD"): // added-not-at-HEAD / deleted tracked → discard (rare; conservative)
			if !sourceWriter {
				continue // leave for the tree-diff guard (non-source phase)
			}
			if err := discardMainLeak(ctx, projectRoot, p); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked main-tree change %s\n", p)
		default: // rename/copy/unknown — not safe to auto-recover
			if !sourceWriter {
				continue // leave for the tree-diff guard (non-source phase)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: unrecoverable leak status %q for %s (falling through to abort)\n", xy, p)
			return false
		}
	}
	if len(relocated) > 0 {
		// -f: a relocated path may be gitignored in the worktree, and a plain
		// `git add` would exit 1 on an ignored path and abort the whole batch.
		args := append([]string{"add", "-f", "--"}, relocated...)
		if _, c, e := gitCapture(ctx, worktree, args...); e != nil || c != 0 {
			// A failed stage leaves the files relocated but invisible to the
			// auditor; return false so the tree-diff guard aborts instead of
			// shipping that half-recovered state.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git add of relocated paths failed (rc=%d): %v — aborting recovery\n", c, e)
			return false
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: %d leaked path(s) relocated into worktree; main tree restored\n", len(relocated))
	}
	return true
}

// moveFile relocates src→dst, falling back to copy+remove when os.Rename
// fails. os.Rename returns EXDEV when src and dst are on different
// filesystems, e.g. when the worktree base resolves to another mount.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, serr := os.Stat(src); serr == nil {
		mode = fi.Mode()
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// relocateTrackedEdit assumes the worktree's copy of p is at HEAD (the
// caller checks), so overlaying the leaked content never clobbers
// independent in-worktree work.
func relocateTrackedEdit(ctx context.Context, projectRoot, worktree, p string) error {
	if err := copyFile(filepath.Join(projectRoot, p), filepath.Join(worktree, p)); err != nil {
		return fmt.Errorf("relocate content of %s: %w", p, err)
	}
	return discardMainLeak(ctx, projectRoot, p) // restore main to HEAD; the worktree now holds the edit
}

// `git checkout HEAD -- p` resets both index and working tree, so it also
// discards a staged-only leak (plain `git checkout -- p` would no-op).
func discardMainLeak(ctx context.Context, projectRoot, p string) error {
	// gitCapture reports a non-zero exit as (c!=0, e==nil); branch instead of
	// wrapping a nil error with %w, which would render "<nil>" and break
	// errors.Is/As.
	_, c, e := gitCapture(ctx, projectRoot, "checkout", "HEAD", "--", p)
	if e != nil {
		return fmt.Errorf("git checkout HEAD -- %s: %w", p, e)
	}
	if c != 0 {
		return fmt.Errorf("git checkout HEAD -- %s: exit %d", p, c)
	}
	return nil
}

func isGitignored(ctx context.Context, dir, p string) bool {
	// git check-ignore -q exits 0 if the path is ignored, 1 if not ignored.
	_, code, err := gitCapture(ctx, dir, "check-ignore", "-q", "--", p)
	return err == nil && code == 0
}

// `git diff --quiet HEAD -- p` exits 0 (clean) / 1 (differs); a launch error
// is treated as "not clean" so the caller falls back to the conservative
// discard path.
func worktreeCleanForPath(ctx context.Context, worktree, p string) bool {
	_, c, e := gitCapture(ctx, worktree, "diff", "--quiet", "HEAD", "--", p)
	return e == nil && c == 0
}

// buildArtifacts are the tracked build outputs a leaked rebuild targets.
// go/bin/evolve is gitignored and normally never appears in `git status`,
// but is listed defensively.
var buildArtifacts = map[string]bool{
	"go/evolve":     true,
	"go/bin/evolve": true,
}

// isLegitimateMainTreePath reports whether a main-tree path is a write
// target that must not trigger the tree-diff guard: tracked build artifacts,
// non-deliverable `.evolve/` state at any nesting depth, and bare directory
// entries from `-uall`. It is the one classification both recoverBuildLeak
// and the boundary guard consult.
func isLegitimateMainTreePath(p string) bool {
	// Build artifacts: a builder may rebuild these into the main tree.
	if buildArtifacts[p] {
		return true
	}
	// Trailing "/" = a bare directory entry (nested worktree/submodule that
	// `-uall` reports without recursing): never a file we can act on.
	if strings.HasSuffix(p, "/") {
		return true
	}
	// `.evolve/` paths at any nesting depth are runtime state — EXCEPT the
	// evolve deliverable prefixes (evals/, phases/, commit-prefix-scope.json)
	// which only flow through worktree-based phases and must never escape.
	if p == ".evolve" || strings.HasPrefix(p, ".evolve/") || strings.Contains(p, "/.evolve/") {
		return !isEvolveDeliverablePath(p)
	}
	return false
}
