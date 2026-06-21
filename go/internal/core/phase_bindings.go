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

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func (o *Orchestrator) emitPhaseBindings(ctx context.Context, cycle int, projectRoot string, cs CycleState, phase Phase, verdict string) {
	in := bindingInputs{
		cycle:       cycle,
		projectRoot: projectRoot,
		workspace:   cs.WorkspacePath,
		worktree:    cs.ActiveWorktree,
		verdict:     verdict,
	}
	switch {
	case phase == PhaseAudit && (verdict == VerdictPASS || verdict == VerdictWARN):
		o.recordPhaseBinding(ctx, phase, in)
	case phase == PhaseBuild && verdict != VerdictSKIPPED:
		o.recordPhaseBinding(ctx, phase, in)
	case o.cfg.PhaseIO >= config.StageEnforce && phase != PhaseAudit && phase != PhaseBuild:
		// Phase 3.9: phase-agnostic binding for user/inserted phases. Dormant
		// until EVOLVE_PHASE_IO=enforce, so the default-off loop emits exactly
		// the audit/build bindings it did before (byte-identical ledger).
		o.recordPhaseBinding(ctx, phase, in)
	}
}

// phaseRole maps a phase to the agent role its provenance ledger entry records.
// audit→auditor and build→builder are the exact role strings ship's audit-binding
// (findLatestAudit) and the rt-001-ledger-role-completeness red-team predicate
// require; every other phase binds under its own name (identity), so a
// user-defined phase gets a stable, predictable role and the identity fallback
// never renames a known agent role. It is a TOTAL map over all phases (so direct
// callers and TestPhaseRole pin the canonical vocabulary); recordPhaseBinding
// itself only routes non-audit/non-build phases through it.
func phaseRole(phase Phase) string {
	switch phase {
	case PhaseAudit:
		return "auditor"
	case PhaseBuild:
		return "builder"
	default:
		return string(phase)
	}
}

// bindingInputs is the filesystem + cycle context a provenance binding needs.
// phaseio.PhaseOutput carries the wire result (verdict/SHAs) but not the
// projectRoot/workspace/worktree the git probes and artifact reads require, so
// the binding takes them explicitly rather than via the typed output.
type bindingInputs struct {
	cycle       int
	projectRoot string
	workspace   string
	worktree    string
	// verdict is consumed only by the audit recorder (verdict→exit_code: WARN→1);
	// build and generic bindings always record exit_code 0.
	verdict string
}

// recordPhaseBinding is the phase-agnostic entry point for provenance bindings.
// audit and build DELEGATE to their specialized recorders UNCHANGED, so their
// ledger bytes stay byte-identical to before (the role vocabulary + fields ship
// depends on — Risk #1). The bodies are genuinely asymmetric (audit alone
// computes a worktree-tree SHA, reads its artifact fatally, derives exit code
// from the verdict, and projects into the verdict cache), so the collapse is at
// the dispatch/role-naming layer, NOT a merged body. Any other phase records a
// generic builder-shaped entry under its identity role; the caller
// (emitPhaseBindings) gates that path to EVOLVE_PHASE_IO=enforce.
func (o *Orchestrator) recordPhaseBinding(ctx context.Context, phase Phase, in bindingInputs) {
	switch phase {
	case PhaseAudit:
		o.recordAuditBinding(ctx, in.cycle, in.projectRoot, in.workspace, in.worktree, in.verdict)
	case PhaseBuild:
		o.recordBuildBinding(ctx, in.cycle, in.projectRoot, in.workspace)
	default:
		o.recordGenericBinding(ctx, phase, in)
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

// recordGenericBinding writes a provenance entry for a phase that has no
// specialized recorder — role = phaseRole(phase) (the phase's identity name).
// It mirrors recordBuildBinding's best-effort shape (git_head + tree_state_sha +
// optional <phase>-report.md artifact), minus the worktree-tree SHA and verdict-
// cache projection that are audit-specific. Reached only at EVOLVE_PHASE_IO=
// enforce (gated by emitPhaseBindings), so user/inserted phases can carry the
// same kind=agent_subprocess provenance audit and build already do. Best-effort:
// any failure WARNs and is swallowed, never blocking the cycle.
func (o *Orchestrator) recordGenericBinding(ctx context.Context, phase Phase, in bindingInputs) {
	head, _, err := gitCapture(ctx, in.projectRoot, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s-binding: git rev-parse HEAD failed: %v\n", phase, err)
		return
	}
	diff, code, derr := gitCapture(ctx, in.projectRoot, "diff", "HEAD")
	if derr != nil || code > 1 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s-binding: git diff HEAD failed (rc=%d): %v\n", phase, code, derr)
		return
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(in.workspace, string(phase)+"-report.md")
	entry := LedgerEntry{
		TS:           o.now().UTC().Format(time.RFC3339),
		Cycle:        in.cycle,
		Role:         phaseRole(phase),
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
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s-binding ledger append: %v\n", phase, err)
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

// normalizeBuildWorktree applies two post-phase normalizations to the active
// worktree, shared by RunCycle and RunCycleFromPhase (resume). The whole
// function is a no-op when there is no active worktree.
//
//  1. Build-commit soft-reset (cycle-156): runs ONLY after PhaseBuild —
//     re-exposes a committing builder's work as pending for audit's
//     `git diff HEAD`. Base comes from the persisted CycleState.WorktreeBaseSHA.
//  2. gofmt -s normalize (cycle-352): runs after EVERY worktree phase, because
//     tdd, build, AND test-amplification all author .go that the audit gofmt
//     gate scans. Cheap no-op when the worktree is already clean.
func (o *Orchestrator) normalizeBuildWorktree(ctx context.Context, completed Phase, cs CycleState) {
	if cs.ActiveWorktree == "" {
		return
	}
	// The build-commit soft-reset (cycle-156) is build-ONLY: it re-exposes a
	// committing builder's work as pending for audit's `git diff HEAD`.
	if completed == PhaseBuild {
		normalizeWorktreeToBase(ctx, cs.ActiveWorktree, cs.WorktreeBaseSHA)
	}
	// The gofmt -s normalize runs after EVERY worktree phase, not just build:
	// tdd, build, AND test-amplification all author .go, and the audit gofmt
	// gate scans the whole worktree. Cycle 352: test-amplification left
	// modeltier_amp_test.go dirty AFTER the build-only normalize, re-failing the
	// gate. Cheap no-op when the worktree is already clean.
	normalizeBuildGofmt(cs.ActiveWorktree)
	// Derived-projection regen is build-ONLY: the flag registry (the SSOT) is
	// edited in the build phase, and the regen is gated on the SSOT actually
	// changing, so non-flag cycles pay no `go run` cost.
	if completed == PhaseBuild {
		o.normalizeDerivedProjections(ctx, cs.ActiveWorktree)
		// Deterministic false-green backstop: run the changed packages' unit
		// tests AFTER the regen (so the tested tree matches what audit binds) and
		// record ground-truth. Best-effort; never aborts — audit is the backstop.
		o.buildSelfCheck(ctx, cs.ActiveWorktree)
	}
}

// normalizeDerivedProjections regenerates each GENERATED projection whose
// source-of-truth this cycle changed (e.g. control-flags.md after a registry
// edit), in the build worktree, BEFORE the audit/docs gate inspects it. Like
// build-gofmt, regenerating a derived projection is deterministic work that must
// NOT depend on the LLM builder remembering: a flag cycle edits registry_table.go
// but the builder routinely leaves the control-flags.md projection stale
// (cycle-11 H1), which the docs/flags gate then correctly FAILs. This closes that
// class at the source — the gate stays the backstop. Best-effort; never aborts.
//
// Timing/integrity: this runs in the BUILD iteration of recordAndBranch (after
// emitPhaseBindings(PhaseBuild), which does NOT compute a tree SHA) and stages the
// regenerated file. The AUDIT iteration's emitPhaseBindings(PhaseAudit) then runs
// worktreeContentSHA (git add -A + write-tree), binding the tree that INCLUDES the
// regenerated projection — so committed_tree == audit_bound_tree holds (no
// CodeIntegrityTreeDrift).
func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {
	if worktree == "" {
		return
	}
	regenStaleProjections(ctx, worktree, changedWorktreePaths(ctx, worktree), regenerateDerivedArtifact, stageWorktreePath)
}

// changedWorktreePaths returns the repo-relative paths this cycle changed in the
// worktree — both tracked changes vs HEAD (`git diff HEAD --name-only`, covering
// staged + unstaged, and post-soft-reset committed work re-exposed as pending) AND
// untracked new files (`git ls-files --others`), so a cycle that ADDS a file under
// a projection's ssotPrefix is not silently missed. The staleness input for
// normalizeDerivedProjections. Split on newlines (NOT strings.Fields) so a path
// containing spaces stays one entry.
func changedWorktreePaths(ctx context.Context, worktree string) []string {
	tracked, _, _ := gitCapture(ctx, worktree, "diff", "HEAD", "--name-only")
	untracked, _, _ := gitCapture(ctx, worktree, "ls-files", "--others", "--exclude-standard")
	var paths []string
	for _, out := range []string{tracked, untracked} {
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			if l != "" {
				paths = append(paths, l)
			}
		}
	}
	return paths
}

// normalizeBuildGofmt applies the deterministic `gofmt -w -s` normalization to
// the build worktree's Go module BEFORE the audit gofmt gate inspects it.
// Formatting is deterministic work and must not depend on the LLM builder
// remembering to run it: when the builder leaves a non-gofmt-s-clean file
// (comment alignment, etc.), the audit gate correctly FAILs the whole cycle
// (cycles 339-341, 350, 351). This closes that class at the source — the gate
// stays the backstop, but the builder's formatting lapses are normalized away
// first. Best-effort: a gofmt failure WARNs and lets the audit gate catch
// anything that slips through; it NEVER aborts the cycle. Scoped to the same
// module dir the audit gate scans (codequality.ModuleDir), so the two cannot
// disagree; the worktree is cut from CI-clean main, so only this cycle's
// changed files are ever dirty.
func normalizeBuildGofmt(worktree string) {
	if worktree == "" {
		return
	}
	fixed, err := codequality.FormatGoFiles(codequality.ModuleDir(worktree))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-gofmt: normalize skipped (%v); audit gofmt gate remains the backstop\n", err)
		return
	}
	if len(fixed) > 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] build-gofmt: ran gofmt -s over %d changed file(s) before audit (gate verifies): %s\n", len(fixed), strings.Join(fixed, ", "))
	}
}

// porcelainDirtySet returns the set of paths `git status --porcelain` reports
// dirty in dir — tracked-modified AND untracked. Captured for the main tree at
// cycle start so recoverBuildLeak only touches paths the BUILD introduced, never
// the operator's pre-existing uncommitted work. (The tree-diff guard's
// `git diff --name-only HEAD` baseline is tracked-only and misses untracked, so
// it can't serve this purpose — see the cycle-160 incident.)
