package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func (o *Orchestrator) emitPhaseBindings(ctx context.Context, cycle int, projectRoot string, cs CycleState, phase Phase, verdict string) {
	if phase == PhaseAudit && (verdict == VerdictPASS || verdict == VerdictWARN) {
		o.recordAuditBinding(ctx, cycle, projectRoot, cs.WorkspacePath, cs.ActiveWorktree, verdict)
	}
	if phase == PhaseBuild && verdict != VerdictSKIPPED {
		o.recordBuildBinding(ctx, cycle, projectRoot, cs.WorkspacePath)
	}
}

// recordAuditBinding writes the rich auditor ledger entry that ship's
// audit-binding (verify.go findLatestAudit / verifyAuditBinding) requires:
// role=auditor, kind=agent_subprocess, with git_head + tree_state_sha +
// artifact_path/sha256. Without it the Go orchestrator recorded audit only as
// kind:phase (no binding fields), so ship fell back to an ancient bash-era
// auditor entry and every cycle failed AUDIT_BINDING_HEAD_MOVED (root cause,
// 2026-05-29). tree_state_sha is sha256(`git diff HEAD`) — byte-identical to
// ship's computeTreeStateSHA so the bind matches. Best-effort: a failure WARNs
// and is swallowed; ship then fails loudly on the missing/stale binding rather
// than shipping unbound.
func (o *Orchestrator) recordAuditBinding(ctx context.Context, cycle int, projectRoot, workspace, worktree, verdict string) {
	head, _, err := gitCapture(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git rev-parse HEAD failed: %v (ship will refuse to bind)\n", err)
		return
	}
	// Worktree CHANGES tree: stage everything (respects .gitignore) and write a
	// tree object = exactly the tree ship will commit. This is what the auditor
	// SHOULD bind (it audited the worktree's working changes); its persona binds
	// HEAD^{tree} = the unchanged base, which can never equal the changes-commit
	// tree → INTEGRITY_TREE_DRIFT every cycle (cycle-152). Ship prefers this
	// over the auditor's comment. Best-effort: empty ⇒ ship falls back to the
	// auditor's value. No commit is made (write-tree only); ship re-stages anyway.
	worktreeTree := worktreeContentSHA(ctx, worktree)
	// `git diff HEAD` returns exit 1 when differences exist — not an error;
	// only exit >1 (e.g. 128) is fatal. Match computeTreeStateSHA semantics.
	diff, code, err := gitCapture(ctx, projectRoot, "diff", "HEAD")
	if err != nil || code > 1 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git diff HEAD failed (rc=%d): %v\n", code, err)
		return
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(workspace, "audit-report.md")
	artBytes, err := os.ReadFile(artPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: read %s: %v\n", artPath, err)
		return
	}
	artSum := sha256.Sum256(artBytes)
	// exit_code mirrors the Unix-convention auditor signal ship tolerates (0|1):
	// 0 = clean PASS, 1 = findings (WARN). Ship's binding accepts both; this
	// keeps the ledger semantically accurate for operators reading it.
	exitCode := 0
	if verdict == VerdictWARN {
		exitCode = 1
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS:              o.now().UTC().Format(time.RFC3339),
		Cycle:           cycle,
		Role:            "auditor",
		Kind:            "agent_subprocess",
		ExitCode:        exitCode,
		GitHEAD:         strings.TrimSpace(head),
		TreeStateSHA:    hex.EncodeToString(treeSum[:]),
		WorktreeTreeSHA: worktreeTree,
		ArtifactPath:    artPath,
		ArtifactSHA256:  hex.EncodeToString(artSum[:]),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding ledger append: %v\n", err)
	}

	// ADR-0048 Slice B: project this verdict into the content-addressed verdict
	// cache, keyed by the SAME worktree tree SHA the binding records. The cache
	// is a projection of the audit binding (single-source), not a second record.
	// Best-effort + advisory: an empty key (no worktree content identity) or a
	// write failure never blocks the cycle — a future lookup miss just costs a
	// full re-run.
	if err := verdictcache.NewStore(projectRoot, o.now).Put(verdictcache.Entry{
		TreeSHA:        worktreeTree,
		Cycle:          cycle,
		Verdict:        verdict,
		ArtifactSHA256: hex.EncodeToString(artSum[:]),
		ArtifactPath:   artPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN verdict-cache put: %v\n", err)
	}
}

// worktreeContentSHA stages the worktree (git add -A, respecting .gitignore) and
// writes a tree object (git write-tree) — the content identity of the cycle's
// changes. It is the SINGLE source for both the audit binding's WorktreeTreeSHA
// (recordAuditBinding) and the ADR-0048 Slice B verdict-cache key, so the value
// recorded and the value looked up are computed identically. Best-effort:
// returns "" when worktree is empty or git fails (callers degrade — ship falls
// back to the auditor comment; the cache simply does not record/match).
func worktreeContentSHA(ctx context.Context, worktree string) string {
	if worktree == "" {
		return ""
	}
	if _, _, aerr := gitCapture(ctx, worktree, "add", "-A"); aerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree content SHA: git add -A failed: %v\n", aerr)
		return ""
	}
	wt, code, werr := gitCapture(ctx, worktree, "write-tree")
	if werr != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree content SHA: git write-tree failed (rc=%d): %v\n", code, werr)
		return ""
	}
	return strings.TrimSpace(wt)
}

// recordBuildBinding writes the builder's provenance ledger entry — role=builder,
// kind=agent_subprocess — that BOTH the red-team predicate rt-001-ledger-role-
// completeness AND the auditor's Ledger-Verification check require as proof the
// builder actually ran. The orchestrator's per-phase entry is role="build" (the
// PHASE name), not "builder" (the AGENT name), and recent cycles no longer get a
// bridge-written per-agent entry — so a cycle that goes through FORMAL audit (vs the
// inline build-commit path that bypasses it) false-FAILed provenance with "no
// role:builder entry" even though the build ran (cycle-181 / issue #13). Mirrors
// recordAuditBinding (role=auditor); best-effort + loud WARN, never blocks the cycle.
func (o *Orchestrator) recordBuildBinding(ctx context.Context, cycle int, projectRoot, workspace string) {
	head, _, err := gitCapture(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-binding: git rev-parse HEAD failed: %v\n", err)
		return
	}
	diff, code, derr := gitCapture(ctx, projectRoot, "diff", "HEAD")
	if derr != nil || code > 1 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-binding: git diff HEAD failed (rc=%d): %v\n", code, derr)
		return
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(workspace, "build-report.md")
	entry := LedgerEntry{
		TS:           o.now().UTC().Format(time.RFC3339),
		Cycle:        cycle,
		Role:         "builder",
		Kind:         "agent_subprocess",
		ExitCode:     0,
		GitHEAD:      strings.TrimSpace(head),
		TreeStateSHA: hex.EncodeToString(treeSum[:]),
		ArtifactPath: artPath,
	}
	if artBytes, rerr := os.ReadFile(artPath); rerr == nil {
		s := sha256.Sum256(artBytes)
		entry.ArtifactSHA256 = hex.EncodeToString(s[:])
	}
	if err := o.ledger.Append(ctx, entry); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-binding ledger append: %v\n", err)
	}
}

// normalizeWorktreeToBase soft-resets the worktree to baseSHA so any commits a
// builder made during the build phase become PENDING changes again. The builder
// is instructed to `git add -A && git commit -m "… [worktree-build]"`
// (agents/evolve-builder.md:235) for crash-safety, but the auditor
// (agents/evolve-auditor.md:57: "Run `git diff HEAD`") and the orchestrator's
// audit-binding (recordAuditBinding: sha256(`git diff HEAD`)) both inspect the
// PENDING diff — which is empty after a commit. agy/Gemini followed the commit
// instruction literally and every cycle's work was discarded as "tree lacks the
// files". Resetting --soft to the cycle base re-exposes the work to `git diff
// HEAD` without changing the auditor prompt or the security binding. See
// docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md (Option C).
//
// Best-effort: any failure WARNs and leaves the worktree untouched (audit then
// inspects whatever state exists); it NEVER aborts the cycle. No-op when HEAD is
// already at baseSHA (the builder left changes uncommitted — the historical
// Claude-builder path), so opting in is byte-identical for non-committing builders.
func normalizeWorktreeToBase(ctx context.Context, worktree, baseSHA string) {
	if worktree == "" || baseSHA == "" {
		return
	}
	head, _, err := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: rev-parse HEAD failed: %v (audit inspects worktree as-is)\n", err)
		return
	}
	if strings.TrimSpace(head) == baseSHA {
		return // builder left changes uncommitted — nothing to normalize
	}
	// Rebase-recovery guard: a PERSISTED base (resume path) can be stale after
	// the operator rebased the cycle worktree onto a moved main. Resetting
	// --soft to a non-ancestor would repoint the branch and stage the entire
	// delta between histories as a spurious diff. Skip instead — the manual
	// recovery already leaves the work pending, so "as-is" is correct.
	if _, code, aerr := gitCapture(ctx, worktree, "merge-base", "--is-ancestor", baseSHA, "HEAD"); aerr != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: base %s is not an ancestor of worktree HEAD (rebase recovery?) — skipping soft-reset, audit inspects worktree as-is\n", baseSHA)
		return
	}
	if _, code, rerr := gitCapture(ctx, worktree, "reset", "--soft", baseSHA); rerr != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: git reset --soft %s failed (rc=%d): %v (audit inspects committed state as-is)\n", baseSHA, code, rerr)
		return
	}
	short := baseSHA
	if len(short) > 12 {
		short = short[:12]
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] worktree-normalize: soft-reset builder commits to base %s — changes now pending for audit\n", short)
}

// normalizeBuildWorktree runs the cycle-156 build-commit normalize at the
// PhaseBuild boundary, shared by RunCycle and RunCycleFromPhase (resume).
// No-op unless the just-completed phase is PhaseBuild with an active
// worktree; the base comes from the persisted CycleState.WorktreeBaseSHA.
func (o *Orchestrator) normalizeBuildWorktree(ctx context.Context, completed Phase, cs CycleState) {
	if completed != PhaseBuild || cs.ActiveWorktree == "" {
		return
	}
	normalizeWorktreeToBase(ctx, cs.ActiveWorktree, cs.WorktreeBaseSHA)
}

// porcelainDirtySet returns the set of paths `git status --porcelain` reports
// dirty in dir — tracked-modified AND untracked. Captured for the main tree at
// cycle start so recoverBuildLeak only touches paths the BUILD introduced, never
// the operator's pre-existing uncommitted work. (The tree-diff guard's
// `git diff --name-only HEAD` baseline is tracked-only and misses untracked, so
// it can't serve this purpose — see the cycle-160 incident.)
