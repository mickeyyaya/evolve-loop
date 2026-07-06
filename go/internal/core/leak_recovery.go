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

// isEvolveDeliverablePath reports whether a TOP-LEVEL `.evolve/` path is a
// deliverable location (nested `<subdir>/.evolve/` runtime residue never is).
// Directory entries (trailing "/") prefix-match their contents; exact-file
// entries require equality — a bare HasPrefix would false-positive
// look-alikes such as `.evolve/commit-prefix-scope.json.bak`.
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

// recoverBuildLeak relocates/discards a main-tree leak from a phase that runs
// with an active worktree, so the cycle continues instead of hard-aborting on
// the tree-diff guard. sourceWriter distinguishes the two recovery regimes:
//   - true  (tdd/build): full recovery — relocate AND stage untracked + tracked
//     edits into the worktree so the auditor's `git diff HEAD` sees builder work.
//   - false (triage/audit/scout/bug-reproduction, added cycle-564 via
//     LeakRecoverablePhase): SAFE SUBSET only — relocate genuine untracked junk
//     out of main (no staging: an artifact leak is not source, and the worktree
//     may not be a git tree for those phases). Evolve deliverables (scout eval
//     materialization) and tracked edits are left UNTOUCHED so the tree-diff
//     guard's own carve-out / abort (with forensics preserved) still governs
//     them. This keeps recovery from becoming a write-permission hole for
//     non-source-writer phases while still clearing the untracked-temp leaks
//     behind the 9 recorded tree-diff-leak failures.
func recoverBuildLeak(ctx context.Context, projectRoot, worktree string, baseline map[string]bool, sourceWriter bool) bool {
	if worktree == "" {
		return true // no worktree to relocate into → degrade (caller guards this anyway)
	}
	// -uall lists untracked FILES individually (never a bare dir), so each leaked
	// path is a file: os.Rename has no dir-collision and is overwrite-safe.
	out, code, err := gitCapture(ctx, projectRoot, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		// Can't determine leaks → DEGRADE to the tree-diff guard (return true, do
		// not abort). false is reserved for a leak we detected but could not safely
		// recover; "couldn't even check" is not that.
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
		// Skip paths isLegitimateMainTreePath classifies as runtime state (R9: one
		// classification path shared with the every-boundary guard check). This
		// covers .evolve/ non-deliverable state at any nesting depth (cycle-176) and
		// bare directory entries from -uall (cycle-1 worktree dir abort).
		// evolveDeliverablePrefixes are NOT skipped — they relocate into the worktree.
		if isLegitimateMainTreePath(p) && !buildArtifacts[p] {
			continue
		}
		xy := line[:2]
		switch {
		case strings.Contains(xy, "?"): // untracked file → relocate out of the main tree
			// A non-source-writer phase that leaks an evolve DELIVERABLE (e.g.
			// scout's eval materialization to .evolve/evals/) must keep it in
			// main — the tree-diff guard's own carve-out (isScoutEvalMaterialization)
			// governs that, not recovery. Only genuine junk leaks relocate.
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
			// Stage into the worktree ONLY for source writers, so the auditor's
			// `git diff HEAD` sees builder SOURCE. A non-source artifact leak
			// just needs to vacate main; staging it as source would be wrong.
			if sourceWriter {
				relocated = append(relocated, p)
			}
		case buildArtifacts[p]: // rebuilt release binary leaked → always discard
			// go/evolve is the marketplace-tracked binary, re-committed ONLY by the
			// release pipeline (releasepipeline.go) and reset to HEAD by the ship phase
			// (ship/gitops.go). A mid-cycle rebuild leaked into the main tree must be
			// discarded, never relocated into the worktree — relocating would commit
			// binary drift (the cycle-153 hazard). Discard regardless of status code.
			if err := discardMainLeak(ctx, projectRoot, p); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked rebuilt artifact %s\n", p)
		case strings.Contains(xy, "M"): // modified tracked file (exists at HEAD)
			if !sourceWriter {
				// A non-source-writer phase must not edit tracked source at all;
				// leave the modification untouched (content preserved for
				// forensics) and let the tree-diff guard abort with its message.
				continue
			}
			// A non-Claude builder may edit an EXISTING tracked source file in the
			// MAIN tree instead of its worktree (cycle-162: orchestrator.go). That is
			// real builder work — preserve it by relocating the leaked content into the
			// worktree, but ONLY when the worktree has not independently modified the
			// same file: a divergent worktree edit is authoritative, and overlaying
			// would clobber it, so discard the main leak in that case.
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
		// Stage ONLY the relocated files (not `git add -A`, which would also stage
		// unrelated worktree content and pollute the auditor's `git diff HEAD`),
		// so the relocated work is visible to audit + the binding — the same
		// visibility reason as normalizeWorktreeToBase. Use -f: a relocated path may
		// be gitignored in the WORKTREE (a builder that edited .gitignore, or a leak
		// that only main's status surfaced) — a plain `git add` exits 1 on an ignored
		// path and would abort the whole batch (cycle-176 / issue #11). The path is a
		// real leak we deliberately moved here for audit, so force-stage it.
		args := append([]string{"add", "-f", "--"}, relocated...)
		if _, c, e := gitCapture(ctx, worktree, args...); e != nil || c != 0 {
			// Fail loudly + return false: the files were physically relocated but
			// are NOT staged, so the auditor's `git diff HEAD` would not see them.
			// Returning false lets the tree-diff guard below abort cleanly rather
			// than ship a half-recovered, audit-invisible state.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git add of relocated paths failed (rc=%d): %v — aborting recovery\n", c, e)
			return false
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: %d leaked path(s) relocated into worktree; main tree restored\n", len(relocated))
	}
	return true
}

// moveFile relocates src→dst, falling back to copy+remove when os.Rename fails.
// os.Rename returns EXDEV when src and dst are on different filesystems — which
// happens when the worktree base resolves to a different volume than projectRoot
// (EVOLVE_WORKTREE_BASE / TMPDIR on another mount). Used by recoverBuildLeak, which
// operates at file granularity (-uall).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile writes src's contents to dst (creating dst's parent dir), preserving src's
// file mode. Shared by moveFile's cross-filesystem fallback and relocateTrackedEdit.
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

// relocateTrackedEdit moves a tracked-file edit that leaked into projectRoot back into
// the worktree: it copies the leaked content over the worktree's copy of p, then
// restores p in the main tree to HEAD (discarding the leak). The caller stages p in the
// worktree afterward (batched with the other relocated paths) so the auditor's
// `git diff HEAD` sees it. Only called when the worktree's copy of p is at HEAD, so the
// overlay never clobbers independent in-worktree work.
func relocateTrackedEdit(ctx context.Context, projectRoot, worktree, p string) error {
	if err := copyFile(filepath.Join(projectRoot, p), filepath.Join(worktree, p)); err != nil {
		return fmt.Errorf("relocate content of %s: %w", p, err)
	}
	return discardMainLeak(ctx, projectRoot, p) // restore main to HEAD; the worktree now holds the edit
}

// discardMainLeak restores p in projectRoot to HEAD, dropping a leaked change. `git
// checkout HEAD -- p` resets BOTH index and working tree, so it discards a staged-only
// ("M "/"A "/"D ") leak too (plain `git checkout -- p` no-ops a staged-only change).
func discardMainLeak(ctx context.Context, projectRoot, p string) error {
	// gitCapture returns a non-zero exit as (c != 0, e == nil); e is non-nil only on a
	// launch failure. Branch so we never wrap a nil error with %w (which would render
	// "<nil>" and break errors.Is/As on the result).
	_, c, e := gitCapture(ctx, projectRoot, "checkout", "HEAD", "--", p)
	if e != nil {
		return fmt.Errorf("git checkout HEAD -- %s: %w", p, e)
	}
	if c != 0 {
		return fmt.Errorf("git checkout HEAD -- %s: exit %d", p, c)
	}
	return nil
}

// isGitignored reports whether the given path is ignored by git in the repository.
func isGitignored(ctx context.Context, dir, p string) bool {
	// git check-ignore -q exits 0 if the path is ignored, 1 if not ignored.
	_, code, err := gitCapture(ctx, dir, "check-ignore", "-q", "--", p)
	return err == nil && code == 0
}

// worktreeCleanForPath reports whether the worktree's copy of p is unmodified from HEAD,
// so overlaying a relocated edit won't clobber independent in-worktree work. `git diff
// --quiet HEAD -- p` exits 0 (clean) / 1 (differs); any launch error is treated as
// "not clean" so the caller falls back to the conservative discard path.
func worktreeCleanForPath(ctx context.Context, worktree, p string) bool {
	_, c, e := gitCapture(ctx, worktree, "diff", "--quiet", "HEAD", "--", p)
	return e == nil && c == 0
}

// buildArtifacts are tracked build outputs a builder may rebuild into the main tree.
// go/evolve is the marketplace-tracked binary, re-committed ONLY by the release pipeline
// (releasepipeline.go) and reset to HEAD by the ship phase (ship/gitops.go); a mid-cycle
// rebuild leaked here must be DISCARDED, never relocated into the worktree (relocating
// would commit binary drift — cycle-153). go/bin/evolve is gitignored and normally never
// appears in `git status`, but is listed defensively.
var buildArtifacts = map[string]bool{
	"go/evolve":     true,
	"go/bin/evolve": true,
}

// isLegitimateMainTreePath reports whether a main-tree path is a legitimate
// write target that must NOT trigger the tree-diff guard abort. Specifically:
//   - tracked build artifacts (binary churn)
//   - `.evolve/` paths at any nesting depth that are NOT evolve deliverables
//     (runtime state: runs/, state.json, ledger.jsonl, instincts/, guards.log,
//     worktrees/ etc.) — the same carve-out inline in recoverBuildLeak's skip
//     branch; `isEvolveDeliverablePath` deliverables (evals/, phases/, commit-
//     prefix-scope.json) are excluded so a worktree-phase leak of those paths
//     still fires the guard
//   - bare directory entries (trailing "/") from `git status --porcelain -uall`
//
// This is the single classification point consulted by BOTH the build-leak
// recovery path (recoverBuildLeak) and the every-boundary guard check (R9).
// See also isScoutEvalMaterialization: a SECOND classifier the boundary guard
// applies for scout's eval-materialization contract — guard-only, because
// scout is not a WorktreePhase and so never reaches recoverBuildLeak.
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

// preserveOnVerdict reports whether a cycle that COMPLETED normally (no abort,
// err==nil) should keep its worktree for salvage based on its final verdict.
// A FAIL verdict (audit FAIL → retro → end) leaves the builder's work
// UNCOMMITTED in the worktree; the default exit-cleanup prune would discard
// it (ADR-0046 Layer 2 was built and lost twice this way, cycles 306/307 —
// inbox preserve-worktree-on-verdict-fail). A PASS/SHIPPED_VIA_BUILD cycle's
// work is already in main and a SKIPPED_UNKNOWN produced nothing, so those
// clean as before. Mirrors the abort-path and ship-failure preservation.
