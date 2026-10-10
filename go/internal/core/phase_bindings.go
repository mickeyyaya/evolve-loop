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
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func (o *Orchestrator) emitPhaseBindings(ctx context.Context, cycle int, projectRoot string, cs CycleState, phase Phase, verdict string) {
	in := bindingInputs{
		cycle:        cycle,
		projectRoot:  projectRoot,
		workspace:    cs.WorkspacePath,
		worktree:     cs.ActiveWorktree,
		worktreeBase: cs.WorktreeBaseSHA,
		verdict:      verdict,
	}
	switch {
	case phase == PhaseAudit && (verdict == VerdictPASS || verdict == VerdictWARN || verdict == VerdictFAIL):
		// FAIL is included: without a binding, ship's lookup has nothing for
		// this run and the FAIL verdict is invisible to the gate built to
		// enforce it. SKIPPED stays excluded (no audit ran, no artifact to bind).
		o.recordPhaseBinding(ctx, phase, in)
	case phase == PhaseBuild && verdict != VerdictSKIPPED:
		o.recordPhaseBinding(ctx, phase, in)
	case o.cfg.PhaseIO >= config.StageEnforce && phase != PhaseAudit && phase != PhaseBuild:
		// Phase-agnostic binding for user/inserted phases. Dormant until
		// EVOLVE_PHASE_IO=enforce, so the default-off loop emits exactly the
		// audit/build bindings it did before (byte-identical ledger).
		o.recordPhaseBinding(ctx, phase, in)
	}
}

// phaseRole maps a phase to the agent role its provenance ledger entry
// records. audit→auditor and build→builder are the exact role strings
// ship's audit-binding (findLatestAudit) and the rt-001-ledger-role-
// completeness red-team predicate require; every other phase binds under
// its own name (identity). It is a TOTAL map over all phases, pinned by
// TestPhaseRole.
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
	// worktreeBase is the worktree's own base commit (CycleState.WorktreeBaseSHA)
	// — the correct operand for the verdict-cache fresh-base guard; projectRoot
	// HEAD at audit time can diverge under fleet concurrency.
	worktreeBase string
	// verdict is consumed only by the audit recorder (verdict→exit_code: WARN→1);
	// build and generic bindings always record exit_code 0.
	verdict string
}

// recordPhaseBinding is the phase-agnostic entry point for provenance bindings.
// audit and build DELEGATE to their specialized recorders UNCHANGED, so their
// ledger bytes stay byte-identical to before. The bodies are genuinely
// asymmetric (audit alone computes a worktree-tree SHA, reads its artifact fatally, derives exit code
// from the verdict, and projects into the verdict cache), so the collapse is at
// the dispatch/role-naming layer, NOT a merged body. Any other phase records a
// generic builder-shaped entry under its identity role; the caller
// (emitPhaseBindings) gates that path to EVOLVE_PHASE_IO=enforce.
func (o *Orchestrator) recordPhaseBinding(ctx context.Context, phase Phase, in bindingInputs) {
	switch phase {
	case PhaseAudit:
		o.recordAuditBinding(ctx, in.cycle, in.projectRoot, in.workspace, in.worktree, in.worktreeBase, in.verdict)
	case PhaseBuild:
		o.recordBuildBinding(ctx, in.cycle, in.projectRoot, in.workspace)
	default:
		o.recordGenericBinding(ctx, phase, in)
	}
}

func auditBindingExitCode(verdict string) int {
	switch verdict {
	case VerdictWARN:
		return 1
	case VerdictFAIL:
		return 2
	default:
		return 0
	}
}

// recordAuditBinding writes the rich auditor ledger entry that ship's
// audit-binding (verify.go findLatestAudit / verifyAuditBinding) requires:
// role=auditor, kind=agent_subprocess, with git_head + tree_state_sha +
// artifact_path/sha256. tree_state_sha is sha256(`git diff HEAD`) —
// byte-identical to ship's computeTreeStateSHA so the bind matches.
// Best-effort: a failure WARNs and is swallowed; ship then fails loudly on
// the missing/stale binding rather than shipping unbound.
func (o *Orchestrator) recordAuditBinding(ctx context.Context, cycle int, projectRoot, workspace, worktree, worktreeBase, verdict string) {
	head, _, err := gitCapture(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git rev-parse HEAD failed: %v (ship will refuse to bind)\n", err)
		return
	}
	// The tree ship will commit from the builder-declared index — not the
	// auditor's own HEAD^{tree}, which can never equal it. Best-effort: empty
	// means ship falls back to the auditor's value. No commit is made
	// (write-tree only); ship re-stages anyway.
	worktreeTree := worktreeContentSHA(ctx, projectRoot, worktree, workspace)
	// `git diff HEAD` returns exit 1 when differences exist — not an error;
	// only exit >1 (e.g. 128) is fatal. Match computeTreeStateSHA semantics.
	diff, code, err := gitCapture(ctx, projectRoot, "diff", "HEAD")
	if err != nil || code > 1 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: git diff HEAD failed (rc=%d): %v\n", code, err)
		return
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(workspace, phasecontract.ArtifactFilename(string(PhaseAudit)))
	artBytes, err := os.ReadFile(artPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding: read %s: %v\n", artPath, err)
		return
	}
	artSum := sha256.Sum256(artBytes)
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS:              o.now().UTC().Format(time.RFC3339),
		Cycle:           cycle,
		Role:            "auditor",
		Kind:            "agent_subprocess",
		ExitCode:        auditBindingExitCode(verdict),
		GitHEAD:         strings.TrimSpace(head),
		TreeStateSHA:    hex.EncodeToString(treeSum[:]),
		WorktreeTreeSHA: worktreeTree,
		WorktreeBaseSHA: worktreeBase,
		ArtifactPath:    artPath,
		ArtifactSHA256:  hex.EncodeToString(artSum[:]),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-binding ledger append: %v\n", err)
	}

	if !verdictcache.Reusable(verdict) {
		return
	}
	if worktreeBase == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN verdict-cache put skipped (cycle %d): no worktree base identity\n", cycle)
		return
	}
	baseTree := worktreeBaseTreeSHA(ctx, worktree, worktreeBase)
	if baseTree == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN verdict-cache put skipped (cycle %d): worktree base %s did not resolve to a tree\n", cycle, worktreeBase)
		return
	}
	if verdictcache.ProbeEligible(baseTree, worktreeTree) {
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
}

// worktreeContentSHA stages tracked worktree changes in the real index, then
// returns the tree Ship will commit (shipmanifest.TakeShipTree): the index,
// every tracked edit, and the paths the cycle's reports declare, untracked or
// not. Undeclared untracked files are not adopted. It is the SINGLE source for
// the audit binding's WorktreeTreeSHA (recordAuditBinding), the composition
// entries' TreeStateSHA and the ADR-0048 Slice B verdict-cache key, and it is
// the tree the audit seals, so every value recorded, looked up and verified
// is computed identically. Best-effort: returns "" when the worktree or the
// workspace is empty or git fails (callers degrade — ship falls back to the
// auditor comment; the cache simply does not record/match).
func worktreeContentSHA(ctx context.Context, projectRoot, worktree, workspace string) string {
	// The operator's index is never staged by a cycle (inPlaceWorktree): the
	// binding degrades to empty and ship falls back to the auditor's value.
	if worktree == "" || workspace == "" || inPlaceWorktree(worktree, projectRoot) {
		return ""
	}
	if _, _, aerr := gitCapture(ctx, worktree, "add", "-u"); aerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree content SHA: git add -u failed: %v\n", aerr)
		return ""
	}
	snap, err := shipmanifest.TakeShipTree(ctx, gitReadIn(ctx, worktree), worktree, workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree content SHA: ship tree: %v\n", err)
		return ""
	}
	return snap.Tree
}

func gitReadIn(ctx context.Context, dir string) shipmanifest.GitRead {
	return func(args ...string) (string, error) {
		out, code, err := gitCapture(ctx, dir, args...)
		if err == nil && code > 1 {
			err = fmt.Errorf("git %s exited %d", strings.Join(args, " "), code)
		}
		return out, err
	}
}

// worktreeBaseTreeSHA resolves the tree SHA of the base commit for the worktree.
// If baseCommit is empty, it falls back to resolving HEAD^{tree}.
// Returns "" on error.
func worktreeBaseTreeSHA(ctx context.Context, worktree, baseCommit string) string {
	if worktree == "" {
		return ""
	}
	ref := "HEAD"
	if baseCommit != "" {
		ref = strings.TrimSpace(baseCommit)
	}
	wt, code, err := gitCapture(ctx, worktree, "rev-parse", ref+"^{tree}")
	if err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree base tree SHA: git rev-parse failed (rc=%d): %v\n", code, err)
		return ""
	}
	return strings.TrimSpace(wt)
}

// recordBuildBinding writes the builder's provenance ledger entry — role=builder,
// kind=agent_subprocess — that BOTH the red-team predicate rt-001-ledger-role-
// completeness AND the auditor's Ledger-Verification check require as proof the
// builder actually ran. The orchestrator's per-phase entry is role="build" (the
// PHASE name), not "builder" (the AGENT name), so a cycle that goes through
// FORMAL audit (vs the inline build-commit path that bypasses it) would
// otherwise false-FAIL provenance with "no role:builder entry" even though the
// build ran. Mirrors recordAuditBinding (role=auditor); best-effort + loud
// WARN, never blocks the cycle.
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
	artPath := filepath.Join(workspace, phasecontract.ArtifactFilename(string(PhaseBuild)))
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
	artPath := filepath.Join(in.workspace, phasecontract.ArtifactFilename(string(phase)))
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
// builder made during the build phase become PENDING changes again. The
// builder is instructed to commit for crash-safety, but the auditor and the
// orchestrator's audit-binding both inspect the PENDING diff, which is empty
// after a commit. Resetting --soft to the cycle base re-exposes the work to
// `git diff HEAD` without changing the auditor prompt or the security
// binding.
//
// Best-effort: any failure WARNs and leaves the worktree untouched (audit then
// inspects whatever state exists); it NEVER aborts the cycle. No-op when HEAD is
// already at baseSHA (the builder left changes uncommitted), so opting in is
// byte-identical for non-committing builders.
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
		return
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
//  1. Build-commit soft-reset: runs ONLY after PhaseBuild — re-exposes a
//     committing builder's work as pending for audit's `git diff HEAD`. Base
//     comes from the persisted CycleState.WorktreeBaseSHA.
//  2. gofmt -s normalize: runs after EVERY worktree phase, because tdd,
//     build, AND test-amplification all author .go that the audit gofmt
//     gate scans. Cheap no-op when the worktree is already clean.
func (o *Orchestrator) normalizeBuildWorktree(ctx context.Context, completed Phase, cs CycleState, projectRoot string) {
	// An in-place worktree is the operator's tree: no soft reset, no gofmt -w,
	// no projection regen — the review sees the root exactly as the operator
	// left it (announced once at provisioning; inPlaceWorktree).
	if cs.ActiveWorktree == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return
	}
	if completed == PhaseBuild {
		normalizeWorktreeToBase(ctx, cs.ActiveWorktree, cs.WorktreeBaseSHA)
	}
	normalizeBuildGofmt(cs.ActiveWorktree)
	// Derived-projection regen is build-ONLY: the flag registry (the SSOT) is
	// edited in the build phase, and the regen is gated on the SSOT actually
	// changing, so non-flag cycles pay no `go run` cost.
	if completed == PhaseBuild {
		o.normalizeDerivedProjections(ctx, cs.ActiveWorktree)
		// Derive the covering-test corpus for test-amplification (inserted after
		// build) so the agent is TOLD its in-scope tests instead of Grepping the
		// whole repo for them. Best-effort: writes nothing when underivable.
		writeCoveringTests(ctx, cs.ActiveWorktree, cs.WorkspacePath)
		// Deterministic false-green backstop: run the changed packages' unit
		// tests AFTER the regen (so the tested tree matches what audit binds) and
		// record ground-truth. Best-effort; never aborts — audit is the backstop.
		// With the build-floor reviewer ENFORCED the same selfcheck already ran
		// (base-diff scope) and wrote the artifact at the review seam — skip the
		// duplicate go-test pass; the gofmt/regen normalizes above still ran.
		if !o.workflowConfig.BuildFloorEnforced {
			o.buildSelfCheck(ctx, cs.ActiveWorktree)
		}
	}
	// Runs after EVERY worktree phase, not just build: test-amplification is
	// inserted after build, but a RESUME past build never sees PhaseBuild
	// complete in this process, so the branch above cannot fire and the phase
	// would silently degrade to an unscoped whole-repo search. No-op (no stat
	// miss, no derivation) once the corpus exists, so a normal cycle pays this
	// exactly once.
	ensureCoveringTests(ctx, cs.ActiveWorktree, cs.WorkspacePath)
}

func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {
	if worktree == "" {
		return
	}
	stale := derived.Stale(changedWorktreePaths(ctx, worktree), goInputsFor(ctx, worktree))
	regenStaleProjections(ctx, worktree, stale, refreshDerivedEntry, stageDerivedEntry)
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

// ChangedWorktreePaths is the exported projection of changedWorktreePaths for
// consumers outside this package — today `evolve phase verify build`, which
// feeds the set to deliverable.VerifyBuildWithChangedPaths so the agent's
// self-check judges the SAME diff the host-side docs-floor reviewer does
// (build_floor_reviewer.go:136). A projection, not a second implementation:
// re-deriving the diff in internal/cli/phasecmd would put two answers to "what
// did this cycle change?" in the tree and let the gate and the self-check drift.
// Fail-open semantics are inherited — a path
// that is not a git repo yields no paths rather than an error.
// See ADR-0034.
func ChangedWorktreePaths(ctx context.Context, worktree string) []string {
	return changedWorktreePaths(ctx, worktree)
}

// normalizeBuildGofmt applies the deterministic `gofmt -w -s` normalization to
// the build worktree's Go module BEFORE the audit gofmt gate inspects it.
// Formatting is deterministic work and must not depend on the LLM builder
// remembering to run it: when the builder leaves a non-gofmt-s-clean file
// (comment alignment, etc.), the audit gate correctly FAILs the whole cycle.
// This closes that class at the source — the gate
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
