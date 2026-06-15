package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func defaultGitHEAD() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN git HEAD probe failed (cycle outcome labels degraded): %v\n", err)
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// emitPhaseBindings writes the per-agent provenance ledger entries ship's
// verification requires after a phase completes (see recordAuditBinding /
// recordBuildBinding for the per-entry contracts). Shared by RunCycle and
// RunCycleFromPhase — the resume path originally skipped these, so a resumed
// audit→ship bound to a stale auditor entry from an earlier cycle and always
// failed AUDIT_BINDING_HEAD_MOVED (cycle-294 incident, 2026-06-12).
// Best-effort: failures are logged to stderr; ship then refuses to bind.

func porcelainDirtySet(ctx context.Context, dir string) map[string]bool {
	set := map[string]bool{}
	// -uall lists every untracked FILE individually (never a bare directory), so
	// recoverBuildLeak relocates at file granularity — no dir-rename ENOTEMPTY in
	// a real worktree, and the baseline is file-exact.
	out, code, err := gitCapture(ctx, dir, "status", "--porcelain", "-uall")
	if err != nil || code != 0 {
		return set
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		set[porcelainPath(line)] = true
		// A rename/copy dirties BOTH sides: without the old path, a deliverable
		// renamed to a look-alike name would vanish from the tree-diff guard's
		// view (the new path classifies as legitimate runtime state).
		if old := porcelainOldPath(line); old != "" {
			set[old] = true
		}
	}
	return set
}

// porcelainPath extracts the path from a `git status --porcelain` line. Lines are
// "XY <path>"; a rename/copy is "XY <old> -> <new>" (take the new path). Quotes
// (paths with special chars) are trimmed best-effort.
func porcelainPath(line string) string {
	p := strings.TrimSpace(line[3:])
	if i := strings.Index(p, " -> "); i >= 0 {
		p = p[i+4:]
	}
	return strings.Trim(p, "\"")
}

// porcelainOldPath extracts the rename/copy SOURCE from a porcelain line
// ("XY <old> -> <new>"), or "" for non-rename lines.
func porcelainOldPath(line string) string {
	p := strings.TrimSpace(line[3:])
	i := strings.Index(p, " -> ")
	if i < 0 {
		return ""
	}
	return strings.Trim(p[:i], "\"")
}

// recoverBuildLeak relocates build-phase writes that escaped into the main tree
// back into the worktree, then restores the main tree — the self-heal for the
// cycle-160 incident (Option A). Non-Claude builders (agy/codex in tmux) are not
// bound by the Claude-only role-gate, and the OS sandbox is off on nested-macOS,
// so they can write to project_root instead of the worktree. Rather than abort
// the cycle, move the build's output to where audit/ship expect it.
//
// baseline (file-granular via `git status --porcelain -uall`) = paths already
// dirty in projectRoot before the build (operator / pre-existing work) — never
// touched. For each NEW dirty path:
//   - untracked ('?')                 → os.Rename(projectRoot/p → worktree/p);
//     the relocated paths (and ONLY those) are then `git add --`'d in the worktree
//     so the auditor's `git diff HEAD` sees them without sweeping in unrelated
//     worktree content (same visibility reason as normalizeWorktreeToBase).
//   - rebuilt release binary (buildArtifacts: go/evolve, go/bin/evolve) → always
//     discard (git checkout HEAD -- p); never relocate, or the cycle would commit
//     binary drift (cycle-153). go/evolve is re-committed only by the release pipeline.
//   - modified tracked ('M') → real builder work edited in the MAIN tree (cycle-162:
//     orchestrator.go). If the worktree has NOT independently touched p (its copy is
//     at HEAD) → relocate the leaked content into the worktree (preserve the work) +
//     stage it. If the worktree diverged for p → discard the main leak (worktree is
//     authoritative).
//   - added/deleted tracked ('A'/'D') → git checkout HEAD -- p (discards staged AND
//     unstaged; plain `git checkout -- p` would no-op a staged-only change).
//   - rename/copy/other → not safe to auto-recover → return false.
//
// Returns true iff every NEW leak was handled and the main tree is clean of them;
// the caller continues. On false the caller ABORTS the cycle — the tree-diff
// guard only backstops tracked leaks, so an unrecovered (esp. untracked) leak
// must not be allowed to slip past into audit. "Couldn't determine" cases degrade
// to true (let the guard be the backstop). Best-effort + loud WARNs throughout.
// evolveDeliverablePrefixes are the `.evolve/` subpaths that are REPO CONTENT
// — locations agents legitimately write as cycle deliverables. A leak under
// one of these relocates into the worktree like any other repo path
// (cycle-262: tracked commit-prefix-scope.json edit; cycle-268: a NEW eval
// file — both previously hit the blanket `.evolve/` skip, were unrecoverable,
// and killed their cycles at the tree-diff guard). Everything else under
// `.evolve/` is never relocated: runtime state (runs/, worktrees/,
// state.json, ledger.jsonl, instincts/, nested guards.log — pinned by the
// Skips tests) AND the TRUST-SENSITIVE operator-privilege documents
// (.evolve/profiles/, .evolve/policy.json) — those configure the gates and
// the auditor's own constraints, so an agent write there must stay
// unrecoverable (the guard kills the cycle, forcing human review; an auditor
// cannot safely review the file that redefines the auditor).

func gitCapture(ctx context.Context, dir string, args ...string) (string, int, error) {
	var buf strings.Builder
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr // surface git diagnostics (fatal: not a repo, …) for triage
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee.ExitCode(), nil
		}
		return "", -1, err
	}
	return buf.String(), 0, nil
}

// defaultGitDirtyPaths runs `git status --porcelain -uall` in repoRoot and
// returns the list of dirty paths (tracked-modified AND untracked), one per
// entry. Porcelain granularity is required for the tree-diff guard to catch
// NEW UNTRACKED files written by inserted/non-worktree phases — the tracked-
// only `git diff --name-only HEAD` baseline that preceded this missed them
// (the cycle-270 root cause). Errors propagate so the guard degrades to
// "snapshot missed" rather than misreport leaks.
func defaultGitDirtyPaths(ctx context.Context, repoRoot string) ([]string, error) {
	set := porcelainDirtySet(ctx, repoRoot)
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

// finalizeOutcome translates SKIPPED into a more specific CycleOutcome label
// using HEAD movement and retro text as signals. PASS/FAIL/WARN pass through.
