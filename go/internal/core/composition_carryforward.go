// composition_carryforward.go — wires the RUNG 0 composition-verdict writer
// into the live fleet-rebase recovery path (cycle 801, inbox weight 0.98,
// campaign merge-efficiency-2026-07).
//
// Ship's trivial-rebase carry-forward reader (internal/phases/ship/
// composition.go) and the ledger's kernel-recomputable writer
// (internal/adapters/ledger/composition.go) were built and unit-tested in
// cycle-786, but no production call site ever produced a composition-verdict
// entry: recoverFromShipError's clean fleet-rebase branch always fell
// through to a full re-audit. internal/core cannot import
// internal/adapters/ledger directly (ledger already imports core — an import
// cycle), so this wires the same Option-injected-closure seam core already
// uses for catalogRefresh/modelCatalogLookup/directivesProvider: the
// composition root (cmd/evolve) binds the closures to the real ledger
// adapter, and core stays adapter-agnostic.
package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

// CompositionAuditSnapshot is what the bound audit reviewed for this lane
// BEFORE a peer moved main — the pre-rebase state a clean fleet rebase's
// composed diff must match (by patch-id) for the carry-forward fast path to
// apply.
type CompositionAuditSnapshot struct {
	LaneAuditRef string // artifact_sha256 of the bound auditor entry
	AuditedBase  string // git HEAD the audit originally bound
	Diff         []byte // unified diff the audit reviewed
	PatchID      string // patch-id of Diff
}

// CompositionVerdictInput mirrors ledger.CompositionVerdictInput field-for-
// field so the composition root's injected writer closure is a 1:1
// translation into a real ledger.WriteCompositionVerdict call, with zero new
// validation logic duplicated in core.
type CompositionVerdictInput struct {
	Cycle        int
	LaneAuditRef string
	PatchID      string
	AuditedBase  string
	GitHead      string
	TreeStateSHA string
	GateResults  map[string]string
	AuditedDiff  []byte
	ComposedDiff []byte
	ArtifactDir  string
}

// compositionArtifactDirName is the worktree-relative directory the fast
// path persists its composition diff artifacts under, mirroring the
// ledger.jsonl symlink convention in linkGuardDeps.
const compositionArtifactDirName = "composition-artifacts"

// WithCompositionSnapshot injects the closure that captures the lane's
// pre-rebase audited state (what the bound audit reviewed). Nil (default)
// keeps the composition fast path off — recovery behaves exactly as it does
// today.
func WithCompositionSnapshot(fn func(ctx context.Context, worktree string) (CompositionAuditSnapshot, error)) Option {
	return func(o *Orchestrator) { o.compositionSnapshot = fn }
}

// WithCompositionGateRunner injects the closure that runs the full native
// composed-tree gate set (ciparity.RequiredComposedGates) against the
// rebased worktree. Nil (default) keeps the composition fast path off.
func WithCompositionGateRunner(fn func(ctx context.Context, worktree string) map[string]string) Option {
	return func(o *Orchestrator) { o.compositionGateRunner = fn }
}

// WithCompositionVerdictWriter injects the closure that persists a
// composition-verdict entry (the composition root binds this to
// ledger.WriteCompositionVerdict). Nil (default) keeps the composition fast
// path off.
func WithCompositionVerdictWriter(fn func(ledgerPath string, in CompositionVerdictInput) error) Option {
	return func(o *Orchestrator) { o.compositionVerdictWriter = fn }
}

// compositionCarryForward attempts the RUNG 0 fast path after a CLEAN fleet
// rebase: if the composed (post-rebase) diff's recomputed patch-id matches
// the pre-rebase audited snapshot AND every required composed-tree gate is
// green, it writes a composition-verdict entry and reports true so recovery
// can route straight back to ship, skipping the full re-audit. Any missing
// seam, patch-id drift, red gate, or writer error returns false — the
// pre-existing full re-audit route is untouched (this can only narrow, never
// widen, what ships).
func (o *Orchestrator) compositionCarryForward(ctx context.Context, cycle int, cs CycleState) bool {
	if o.compositionSnapshot == nil || o.compositionGateRunner == nil || o.compositionVerdictWriter == nil {
		return false
	}
	worktree := cs.ActiveWorktree
	if worktree == "" {
		return false
	}
	snap, err := o.compositionSnapshot(ctx, worktree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: snapshot unavailable: %v; falling back to full re-audit\n", err)
		return false
	}
	composedDiff, exit, err := gitCapture(ctx, worktree, "diff", "main...HEAD")
	if err != nil || exit != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: composed diff unavailable (exit=%d, err=%v); falling back to full re-audit\n", exit, err)
		return false
	}
	patchID, err := compositionPatchID([]byte(composedDiff))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: patch-id computation failed: %v; falling back to full re-audit\n", err)
		return false
	}
	if patchID != snap.PatchID {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: composed patch-id %s does not match audited %s (semantic drift); falling back to full re-audit\n", patchID, snap.PatchID)
		return false
	}
	gateResults := o.compositionGateRunner(ctx, worktree)
	if missing := ciparity.MissingComposedGates(gateResults); missing != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: composed-tree gates not green (%s); falling back to full re-audit\n", strings.Join(missing, ","))
		return false
	}
	gitHead, _, _ := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	in := CompositionVerdictInput{
		Cycle:        cycle,
		LaneAuditRef: snap.LaneAuditRef,
		PatchID:      patchID,
		AuditedBase:  snap.AuditedBase,
		GitHead:      strings.TrimSpace(gitHead),
		TreeStateSHA: worktreeContentSHA(ctx, worktree),
		GateResults:  gateResults,
		AuditedDiff:  snap.Diff,
		ComposedDiff: []byte(composedDiff),
		ArtifactDir:  filepath.Join(worktree, ".evolve", compositionArtifactDirName),
	}
	ledgerPath := filepath.Join(worktree, ".evolve", "ledger.jsonl")
	if err := o.compositionVerdictWriter(ledgerPath, in); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: writer failed (fail-closed): %v; falling back to full re-audit\n", err)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] composition carry-forward: wrote composition-verdict for cycle %d; skipping re-audit\n", cycle)
	return true
}

// compositionPatchID pipes a unified diff through `git patch-id --stable`
// and returns the patch-id — mirrors ledger.PatchID (internal/core cannot
// import internal/adapters/ledger, which already imports core).
func compositionPatchID(diff []byte) (string, error) {
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Stdin = bytes.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("git patch-id --stable: empty output (empty diff?)")
	}
	return fields[0], nil
}
