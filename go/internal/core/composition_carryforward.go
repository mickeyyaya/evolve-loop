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

// CompositionAuditSnapshot is what the bound audit reviewed before a peer moved main.
type CompositionAuditSnapshot struct {
	LaneAuditRef string // artifact_sha256 of the bound auditor entry
	AuditedBase  string // git HEAD the audit originally bound
	Diff         []byte // unified diff the audit reviewed
	PatchID      string // patch-id of Diff
}

// CompositionVerdictInput mirrors ledger.CompositionVerdictInput field for field; core cannot import the ledger adapter.
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

const compositionArtifactDirName = "composition-artifacts"

// WithCompositionSnapshot injects the capture of the lane's pre-rebase audited state; nil keeps the fast path off.
func WithCompositionSnapshot(fn func(ctx context.Context, worktree, runID string) (CompositionAuditSnapshot, error)) Option {
	return func(o *Orchestrator) { o.compositionSnapshot = fn }
}

// WithCompositionGateRunner injects the composed-tree gate run over the rebased worktree; nil keeps the fast path off.
func WithCompositionGateRunner(fn func(ctx context.Context, worktree string) map[string]string) Option {
	return func(o *Orchestrator) { o.compositionGateRunner = fn }
}

// WithCompositionVerdictWriter injects the composition-verdict ledger writer; nil keeps the fast path off.
func WithCompositionVerdictWriter(fn func(ledgerPath string, in CompositionVerdictInput) error) Option {
	return func(o *Orchestrator) { o.compositionVerdictWriter = fn }
}

// CompositionFastPathWired reports whether all three composition closures are bound; a partial binding is not wired.
func (o *Orchestrator) CompositionFastPathWired() bool {
	return o.compositionSnapshot != nil &&
		o.compositionGateRunner != nil &&
		o.compositionVerdictWriter != nil
}

// compositionCarryForward is RUNG 0: a clean rebase whose composed patch-id matches the
// audited one and whose gates are green reships without a re-audit. Any miss returns false.
func (o *Orchestrator) compositionCarryForward(ctx context.Context, cycle int, cs CycleState, projectRoot string) bool {
	if o.compositionSnapshot == nil || o.compositionGateRunner == nil || o.compositionVerdictWriter == nil {
		return false
	}
	worktree := cs.ActiveWorktree
	if worktree == "" {
		return false
	}
	snap, err := o.compositionSnapshot(ctx, worktree, cs.RunID)
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
		TreeStateSHA: worktreeContentSHA(ctx, projectRoot, worktree),
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

// WithScopedMergeReviewer injects the RUNG 2 reviewer; nil sends a RUNG 0 miss straight to the full re-audit.
func WithScopedMergeReviewer(fn ScopedMergeReviewer) Option {
	return func(o *Orchestrator) { o.scopedMergeReviewer = fn }
}

// ScopedMergeReviewWired reports whether the composition root bound the RUNG 2 reviewer.
func (o *Orchestrator) ScopedMergeReviewWired() bool {
	return o.scopedMergeReviewer != nil
}

// scopedMergeCarryForward is RUNG 2: after a RUNG 0 miss it reviews only the intersecting
// hunks. Only a compatible verdict whose resolution re-verifies composes; any miss returns false.
func (o *Orchestrator) scopedMergeCarryForward(ctx context.Context, cycle int, cs CycleState, projectRoot string) bool {
	if o.scopedMergeReviewer == nil || o.compositionSnapshot == nil ||
		o.compositionGateRunner == nil || o.compositionVerdictWriter == nil {
		return false
	}
	worktree := cs.ActiveWorktree
	if worktree == "" {
		return false
	}
	snap, err := o.compositionSnapshot(ctx, worktree, cs.RunID)
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
	// A compatible verdict is trusted only when its resolution re-verifies by
	// patch-id against the audited change, never on the reviewer's word.
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
		TreeStateSHA: worktreeContentSHA(ctx, projectRoot, worktree),
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

// compositionPatchID mirrors ledger.PatchID, which core cannot import.
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
