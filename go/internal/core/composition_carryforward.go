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
	Method       string
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

// CompositionFastPathWired reports whether the composition root bound ALL
// THREE composition closures — snapshot, gate runner, and verdict writer.
// It is an AND, not an OR: a partial binding (any nil) leaves
// compositionCarryForward's nil-guard tripping, so it must NOT report itself
// as wired. Mirrors FailureAdviserWired (failure_hook.go) — the same
// observability seam that lets the composition root prove, in a real
// (non-fake) test, that its wiring actually reaches production.
func (o *Orchestrator) CompositionFastPathWired() bool {
	return o.compositionSnapshot != nil &&
		o.compositionGateRunner != nil &&
		o.compositionVerdictWriter != nil
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

// WithScopedMergeReviewer injects the RUNG 2 scoped merge reviewer closure.
// Nil (default) keeps RUNG 2 dark — recovery falls straight from a RUNG 0 miss
// to the RUNG 3 full re-audit, exactly as it does today.
func WithScopedMergeReviewer(fn ScopedMergeReviewer) Option {
	return func(o *Orchestrator) { o.scopedMergeReviewer = fn }
}

// ScopedMergeReviewWired reports whether the composition root bound the RUNG 2
// reviewer closure. Mirrors CompositionFastPathWired — the observability seam
// that lets a real (non-fake) test prove the wiring reaches production.
func (o *Orchestrator) ScopedMergeReviewWired() bool {
	return o.scopedMergeReviewer != nil
}

// scopedMergeCarryForward attempts the RUNG 2 fast path after a RUNG 0 miss
// (the composed patch-id drifted from the audited one — real overlapping
// edits): it dispatches only the intersecting hunks to the injected reviewer.
// A `compatible` disposition whose (optional) resolution re-enters RUNG 0
// patch-id verification writes a composition-verdict{method:"scoped-review"}
// and lets recovery reship; `entangled`, a nil reviewer, any missing seam, a
// red gate, an unverified resolution, or a writer error returns false — the
// pre-existing full re-audit route is untouched (this can only narrow, never
// widen, what ships).
func (o *Orchestrator) scopedMergeCarryForward(ctx context.Context, cycle int, cs CycleState) bool {
	if o.scopedMergeReviewer == nil || o.compositionSnapshot == nil ||
		o.compositionGateRunner == nil || o.compositionVerdictWriter == nil {
		return false
	}
	worktree := cs.ActiveWorktree
	if worktree == "" {
		return false
	}
	snap, err := o.compositionSnapshot(ctx, worktree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: snapshot unavailable: %v; falling back to full re-audit\n", err)
		return false
	}
	composedDiff, exit, err := gitCapture(ctx, worktree, "diff", "main...HEAD")
	if err != nil || exit != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: composed diff unavailable (exit=%d, err=%v); falling back to full re-audit\n", exit, err)
		return false
	}
	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:     snap.Diff,
		ComposedDiff:    []byte(composedDiff),
		AuditedSummary:  "audited change (ref " + snap.LaneAuditRef + ")",
		ComposedSummary: "composed tree after fleet rebase",
	}, o.scopedMergeReviewer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review failed closed: %v; falling back to full re-audit\n", err)
		return false
	}
	if res.Disposition != ScopedMergeCompatible {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: %s — escalating to full re-audit\n", res.Disposition)
		return false
	}
	// MergeBERT invariant: a compatible verdict is trusted only when its
	// resolution re-enters RUNG 0 patch-id verification against the audited
	// change — never on the reviewer's word. The resolution is the reviewer's
	// suggested diff when it supplied one, else the composed diff itself.
	resolution := res.ResolutionDiff
	if len(resolution) == 0 {
		resolution = []byte(composedDiff)
	}
	matches, err := ResolutionMatchesAudited(snap.PatchID, resolution)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: resolution rung-0 re-entry failed: %v; falling back to full re-audit\n", err)
		return false
	}
	if !matches {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: resolution patch-id does not match audited change (unverified); falling back to full re-audit\n")
		return false
	}
	gateResults := o.compositionGateRunner(ctx, worktree)
	if missing := ciparity.MissingComposedGates(gateResults); missing != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: composed-tree gates not green (%s); falling back to full re-audit\n", strings.Join(missing, ","))
		return false
	}
	gitHead, _, _ := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	in := CompositionVerdictInput{
		Cycle:        cycle,
		Method:       scopedReviewMethod,
		LaneAuditRef: snap.LaneAuditRef,
		PatchID:      snap.PatchID,
		AuditedBase:  snap.AuditedBase,
		GitHead:      strings.TrimSpace(gitHead),
		TreeStateSHA: worktreeContentSHA(ctx, worktree),
		GateResults:  gateResults,
		AuditedDiff:  snap.Diff,
		ComposedDiff: resolution,
		ArtifactDir:  filepath.Join(worktree, ".evolve", compositionArtifactDirName),
	}
	ledgerPath := filepath.Join(worktree, ".evolve", "ledger.jsonl")
	if err := o.compositionVerdictWriter(ledgerPath, in); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: writer failed (fail-closed): %v; falling back to full re-audit\n", err)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] scoped merge review: wrote scoped-review composition-verdict for cycle %d; skipping re-audit\n", cycle)
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
