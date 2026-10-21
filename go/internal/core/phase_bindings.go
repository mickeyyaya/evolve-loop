package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
		o.recordPhaseBinding(ctx, phase, in)
	case phase == PhaseBuild && verdict != VerdictSKIPPED:
		o.recordPhaseBinding(ctx, phase, in)
	case o.cfg.PhaseIO >= config.StageEnforce && phase != PhaseAudit && phase != PhaseBuild:
		o.recordPhaseBinding(ctx, phase, in)
	}
}

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

type bindingInputs struct {
	cycle        int
	projectRoot  string
	workspace    string
	worktree     string
	worktreeBase string
	verdict      string
}

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

func (o *Orchestrator) recordAuditBinding(ctx context.Context, cycle int, projectRoot, workspace, worktree, worktreeBase, verdict string) {
	entry, err := auditBindingEntry(ctx, projectRoot, workspace, worktree, worktreeBase, verdict)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN audit-binding: %v\n", err)
		return
	}
	entry.TS = o.now().UTC().Format(time.RFC3339)
	entry.Cycle = cycle
	artPath, worktreeTree, artSHA := entry.ArtifactPath, entry.WorktreeTreeSHA, entry.ArtifactSHA256
	if err := o.ledger.Append(ctx, entry); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN audit-binding ledger append: %v\n", err)
	}

	if !verdictcache.Reusable(verdict) {
		return
	}
	if worktreeBase == "" {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN verdict-cache put skipped (cycle %d): no worktree base identity\n", cycle)
		return
	}
	baseTree := worktreeBaseTreeSHA(ctx, worktree, worktreeBase)
	if baseTree == "" {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN verdict-cache put skipped (cycle %d): worktree base %s did not resolve to a tree\n", cycle, worktreeBase)
		return
	}
	if verdictcache.ProbeEligible(baseTree, worktreeTree) {
		if err := verdictcache.NewStore(projectRoot, o.now).Keep(policy.GCFor(projectRoot).VerdictCacheKeep()).Put(verdictcache.Entry{
			TreeSHA:        worktreeTree,
			Cycle:          cycle,
			Verdict:        verdict,
			ArtifactSHA256: artSHA,
			ArtifactPath:   artPath,
		}); err != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN verdict-cache put: %v\n", err)
		}
	}
}

func auditBindingEntry(ctx context.Context, projectRoot, workspace, worktree, worktreeBase, verdict string) (LedgerEntry, error) {
	head, _, err := gitCapture(ctx, projectRoot, "rev-parse", "HEAD")
	if err != nil {
		return LedgerEntry{}, fmt.Errorf("git rev-parse HEAD failed: %w (ship will refuse to bind)", err)
	}
	worktreeTree := worktreeContentSHA(ctx, projectRoot, worktree, workspace)
	diff, code, err := gitCapture(ctx, projectRoot, "diff", "HEAD")
	if err != nil || code > 1 {
		return LedgerEntry{}, fmt.Errorf("git diff HEAD failed (rc=%d): %v", code, err)
	}
	treeSum := sha256.Sum256([]byte(diff))
	artPath := filepath.Join(workspace, phasecontract.ArtifactFilename(string(PhaseAudit)))
	artBytes, err := os.ReadFile(artPath)
	if err != nil {
		return LedgerEntry{}, fmt.Errorf("read %s: %w", artPath, err)
	}
	artSum := sha256.Sum256(artBytes)
	return LedgerEntry{
		Role:            "auditor",
		Kind:            "agent_subprocess",
		ExitCode:        auditBindingExitCode(verdict),
		GitHEAD:         strings.TrimSpace(head),
		TreeStateSHA:    hex.EncodeToString(treeSum[:]),
		WorktreeTreeSHA: worktreeTree,
		WorktreeBaseSHA: worktreeBase,
		ArtifactPath:    artPath,
		ArtifactSHA256:  hex.EncodeToString(artSum[:]),
	}, nil
}

func worktreeContentSHA(ctx context.Context, projectRoot, worktree, workspace string) string {
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

func normalizeWorktreeToBase(ctx context.Context, worktree, baseSHA string, warn io.Writer) {
	if worktree == "" || baseSHA == "" {
		return
	}
	head, _, err := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN worktree-normalize: rev-parse HEAD failed: %v (audit inspects worktree as-is)\n", err)
		return
	}
	if strings.TrimSpace(head) == baseSHA {
		return
	}
	if _, code, aerr := gitCapture(ctx, worktree, "merge-base", "--is-ancestor", baseSHA, "HEAD"); aerr != nil || code != 0 {
		fmt.Fprintf(warn, "[orchestrator] WARN worktree-normalize: base %s is not an ancestor of worktree HEAD (rebase recovery?) — skipping soft-reset, audit inspects worktree as-is\n", baseSHA)
		return
	}
	if _, code, rerr := gitCapture(ctx, worktree, "reset", "--soft", baseSHA); rerr != nil || code != 0 {
		fmt.Fprintf(warn, "[orchestrator] WARN worktree-normalize: git reset --soft %s failed (rc=%d): %v (audit inspects committed state as-is)\n", baseSHA, code, rerr)
		return
	}
	short := baseSHA
	if len(short) > 12 {
		short = short[:12]
	}
	fmt.Fprintf(warn, "[orchestrator] worktree-normalize: soft-reset builder commits to base %s — changes now pending for audit\n", short)
}

func (o *Orchestrator) normalizeBuildWorktree(ctx context.Context, completed Phase, cs CycleState, projectRoot string) {
	if cs.ActiveWorktree == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return
	}
	if completed == PhaseBuild {
		normalizeWorktreeToBase(ctx, cs.ActiveWorktree, cs.WorktreeBaseSHA, o.errOut())
	}
	normalizeBuildGofmt(cs.ActiveWorktree)
	if completed == PhaseBuild {
		o.normalizeDerivedProjections(ctx, cs.ActiveWorktree)
		writeBuildDiffSummary(ctx, cs.ActiveWorktree, cs.WorktreeBaseSHA, cs.WorkspacePath, o.errOut())
		writeCoveringTests(ctx, cs.ActiveWorktree, cs.WorkspacePath, o.errOut())
		if !o.workflowConfig.BuildFloorEnforced {
			o.buildSelfCheck(ctx, cs.ActiveWorktree)
		}
	}
	ensureCoveringTests(ctx, cs.ActiveWorktree, cs.WorkspacePath, o.errOut())
}

func (o *Orchestrator) normalizeDerivedProjections(ctx context.Context, worktree string) {
	if worktree == "" {
		return
	}
	stale := derived.Stale(changedWorktreePaths(ctx, worktree), goInputsFor(ctx, worktree))
	regenStaleProjections(ctx, worktree, stale, refreshDerivedEntry, stageDerivedEntry)
}

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

func ChangedWorktreePaths(ctx context.Context, worktree string) []string {
	return changedWorktreePaths(ctx, worktree)
}

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
