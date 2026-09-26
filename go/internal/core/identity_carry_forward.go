package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

// identicalRebaseMethod is the carry record's method; cmd pins it equal to the ledger adapter's constant.
const identicalRebaseMethod = "identical-rebase"

// identityCarryForward is ADR-0105 B3. After the rebind proved the pended change identical, it proves the
// same on the trees, runs the composed-tree gates under the fence, records the carry and lets ship take the
// audited verdict; any doubt returns false and the change is audited again.
func (o *Orchestrator) identityCarryForward(ctx context.Context, cycle int, cs CycleState, base0, projectRoot string) bool {
	if !o.CompositionFastPathWired() || cs.ActiveWorktree == "" {
		return false
	}
	decline := func(format string, args ...any) bool {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d identity carry declined: %s; re-auditing\n", cycle, fmt.Sprintf(format, args...))
		return false
	}
	worktree := cs.ActiveWorktree
	audit, err := o.latestAuditEntry(ctx, cs.RunID)
	if err != nil || audit.WorktreeTreeSHA == "" || audit.ArtifactSHA256 == "" {
		return decline("no auditor row names both the tree and the artifact (err=%v)", err)
	}
	if audit.GitHEAD != "" && audit.GitHEAD != base0 {
		return decline("the audit was bound on %s, not on the base %s the change was authored on", audit.GitHEAD, base0)
	}
	tree0 := audit.WorktreeTreeSHA
	base1, err := gitStdout(ctx, gitCapture, worktree, "rev-parse", "HEAD")
	if err != nil {
		return decline("HEAD: %v", err)
	}
	tree1, err := gitStdout(ctx, gitCapture, worktree, "write-tree")
	if err != nil {
		return decline("write-tree: %v", err)
	}
	audited, composed, identical, err := identicalChange(ctx, gitCapture, worktree, base0, tree0, base1, tree1)
	if err != nil || !identical {
		return decline("the pended change is not byte for byte the audited one (err=%v)", err)
	}
	gates, ok := o.gatesOnIntactTree(ctx, worktree, tree1)
	if !ok {
		return decline("the composed-tree gates did not leave the tree as they found it")
	}
	if missing := ciparity.MissingComposedGates(gates); missing != nil {
		return decline("composed-tree gates not green: %v", missing)
	}
	patchID, err := compositionPatchID(audited)
	if err != nil {
		return decline("patch-id: %v", err)
	}
	in := CompositionVerdictInput{
		Cycle: cycle, Method: identicalRebaseMethod, LaneAuditRef: audit.ArtifactSHA256, PatchID: patchID,
		AuditedBase: base0, GitHead: base1, TreeStateSHA: tree1, AuditedTreeSHA: tree0, GateResults: gates,
		AuditedDiff: audited, ComposedDiff: composed, ArtifactDir: filepath.Join(worktree, ".evolve", compositionArtifactDirName),
	}
	if err := o.compositionVerdictWriter(filepath.Join(projectRoot, ".evolve", "ledger.jsonl"), in); err != nil {
		return decline("carry record: %v", err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d rebase is byte-identical: the audited verdict carries, shipping without a second audit (ADR-0105 B3)\n", cycle)
	return true
}

// gatesOnIntactTree runs the composed-tree gates under the fence and reports them only when the tree they ran on
// is the tree ship will commit.
func (o *Orchestrator) gatesOnIntactTree(ctx context.Context, worktree, tree string) (map[string]string, bool) {
	fence := treefence.Begin(ctx, worktree, true)
	gates := o.compositionGateRunner(ctx, worktree)
	outcome := fence.End(ctx)
	if outcome.TakeErr != nil || outcome.RestoreErr != nil {
		return nil, false
	}
	after, err := gitStdout(ctx, gitCapture, worktree, "write-tree")
	if err != nil || after != tree {
		return nil, false
	}
	return gates, true
}
