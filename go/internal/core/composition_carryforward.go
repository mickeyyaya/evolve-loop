package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const CodeComposedGateDeclined signalcenter.Code = "ORCHESTRATOR_COMPOSED_GATE_DECLINED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeComposedGateDeclined,
		"a composed-tree gate did not report pass at a MissingComposedGates trip site (RUNG 0 compositionCarryForward, RUNG 2 scopedMergeCarryForward or the ADR-0105 B3 identityCarryForward); fields.tail_<gate> carries each failing gate's output tail and the cycle falls back to a full re-audit instead of carrying the audit verdict forward")
}

const signalFieldRunes = 512

type gateOutcomeSinkKey struct{}

func publishGateOutcomes(ctx context.Context, outcomes map[string]ciparity.GateOutcome) {
	if sink, ok := ctx.Value(gateOutcomeSinkKey{}).(*map[string]ciparity.GateOutcome); ok {
		*sink = outcomes
	}
}

func (o *Orchestrator) runGateSet(ctx context.Context, worktree string) (map[string]string, map[string]ciparity.GateOutcome) {
	sink := new(map[string]ciparity.GateOutcome)
	statuses := o.compositionGateRunner(context.WithValue(ctx, gateOutcomeSinkKey{}, sink), worktree)
	return statuses, *sink
}

func (o *Orchestrator) declineComposedGates(cycle int, cs CycleState, origin, label string, missing []string, outcomes map[string]ciparity.GateOutcome) {
	gates := strings.Join(missing, ",")
	fields := map[string]string{}
	for _, gate := range missing {
		if tail := outcomes[gate].Tail; tail != "" {
			fields["tail_"+gate] = boundedTail(tail, signalcenter.MaxLineBytes/2/len(missing))
		}
	}
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Module: signalcenter.ModuleOrchestrator,
		Origin: origin, Kind: signalcenter.KindGateRejected, Severity: signalcenter.SeverityWarn,
		Code: CodeComposedGateDeclined, Reason: "composed-tree gates not green (" + gates + ")", Fields: fields,
	})
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d %s: composed-tree gates not green (%s), %s; falling back to full re-audit\n", cycle, label, gates, CodeComposedGateDeclined)
}

func boundedTail(tail string, maxJSONBytes int) string {
	runes := []rune(strings.ToValidUTF8(tail, "\uFFFD"))
	const marker = "…"
	budget, keep := maxJSONBytes-len(marker), len(runes)
	for used := 0; keep > 0; keep-- {
		quoted, _ := json.Marshal(string(runes[keep-1]))
		if used+len(quoted)-2 > budget || len(runes)-keep+1 > signalFieldRunes-1 {
			break
		}
		used += len(quoted) - 2
	}
	if keep == 0 {
		return string(runes)
	}
	return marker + string(runes[keep:])
}

func (o *Orchestrator) runComposedGates(ctx context.Context, cycle int, cs CycleState, worktree, origin, label string) (map[string]string, bool) {
	gateResults, outcomes := o.runGateSet(ctx, worktree)
	missing := ciparity.MissingComposedGates(gateResults)
	if missing == nil {
		return gateResults, true
	}
	o.declineComposedGates(cycle, cs, origin, label, missing, outcomes)
	return nil, false
}

type CompositionAuditSnapshot struct {
	LaneAuditRef string
	AuditedBase  string
	Diff         []byte
	PatchID      string
}

type CompositionVerdictInput struct {
	Cycle          int
	Method         string
	LaneAuditRef   string
	PatchID        string
	AuditedBase    string
	GitHead        string
	TreeStateSHA   string
	AuditedTreeSHA string
	GateResults    map[string]string
	AuditedDiff    []byte
	ComposedDiff   []byte
}

func WithCompositionSnapshot(fn func(ctx context.Context, worktree, runID string) (CompositionAuditSnapshot, error)) Option {
	return func(o *Orchestrator) { o.compositionSnapshot = fn }
}

func WithCompositionGateRunner(fn func(ctx context.Context, worktree string) map[string]ciparity.GateOutcome) Option {
	return func(o *Orchestrator) {
		o.compositionGateRunner = func(ctx context.Context, worktree string) map[string]string {
			outcomes := fn(ctx, worktree)
			publishGateOutcomes(ctx, outcomes)
			return ciparity.GateStatuses(outcomes)
		}
	}
}

func WithCompositionVerdictWriter(fn func(ledgerPath string, in CompositionVerdictInput) error) Option {
	return func(o *Orchestrator) { o.compositionVerdictWriter = fn }
}

func (o *Orchestrator) CompositionFastPathWired() bool {
	return o.compositionSnapshot != nil &&
		o.compositionGateRunner != nil &&
		o.compositionVerdictWriter != nil
}

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
		fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: snapshot unavailable: %v; falling back to full re-audit\n", err)
		return false
	}
	composedDiff, exit, err := gitCapture(ctx, worktree, "diff", "main...HEAD")
	if err != nil || exit != 0 {
		fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: composed diff unavailable (exit=%d, err=%v); falling back to full re-audit\n", exit, err)
		return false
	}
	patchID, err := compositionPatchID([]byte(composedDiff))
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: patch-id computation failed: %v; falling back to full re-audit\n", err)
		return false
	}
	if patchID != snap.PatchID {
		fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: composed patch-id %s does not match audited %s (semantic drift); falling back to full re-audit\n", patchID, snap.PatchID)
		return false
	}
	gateResults, ok := o.runComposedGates(ctx, cycle, cs, worktree, "Orchestrator.compositionCarryForward", "composition carry-forward")
	if !ok {
		return false
	}
	gitHead, _, _ := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	in := CompositionVerdictInput{
		Cycle:        cycle,
		LaneAuditRef: snap.LaneAuditRef,
		PatchID:      patchID,
		AuditedBase:  snap.AuditedBase,
		GitHead:      strings.TrimSpace(gitHead),
		TreeStateSHA: worktreeContentSHA(ctx, projectRoot, worktree, cs.WorkspacePath),
		GateResults:  gateResults,
		AuditedDiff:  snap.Diff,
		ComposedDiff: []byte(composedDiff),
	}
	ledgerPath := filepath.Join(projectRoot, ".evolve", "ledger.jsonl")
	if err := o.compositionVerdictWriter(ledgerPath, in); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: writer failed (fail-closed): %v; falling back to full re-audit\n", err)
		return false
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] composition carry-forward: wrote composition-verdict for cycle %d; skipping re-audit\n", cycle)
	return true
}

func WithScopedMergeReviewer(fn ScopedMergeReviewer) Option {
	return func(o *Orchestrator) { o.scopedMergeReviewer = fn }
}

func (o *Orchestrator) ScopedMergeReviewWired() bool {
	return o.scopedMergeReviewer != nil
}

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
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: snapshot unavailable: %v; falling back to full re-audit\n", err)
		return false
	}
	composedDiff, exit, err := gitCapture(ctx, worktree, "diff", "main...HEAD")
	if err != nil || exit != 0 {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: composed diff unavailable (exit=%d, err=%v); falling back to full re-audit\n", exit, err)
		return false
	}
	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:     snap.Diff,
		ComposedDiff:    []byte(composedDiff),
		AuditedSummary:  "audited change (ref " + snap.LaneAuditRef + ")",
		ComposedSummary: "composed tree after fleet rebase",
	}, o.scopedMergeReviewer)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review failed closed: %v; falling back to full re-audit\n", err)
		return false
	}
	if res.Disposition != ScopedMergeCompatible {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: %s — escalating to full re-audit\n", res.Disposition)
		return false
	}
	resolution := res.ResolutionDiff
	if len(resolution) == 0 {
		resolution = []byte(composedDiff)
	}
	matches, err := ResolutionMatchesAudited(snap.PatchID, resolution)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: resolution rung-0 re-entry failed: %v; falling back to full re-audit\n", err)
		return false
	}
	if !matches {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: resolution patch-id does not match audited change (unverified); falling back to full re-audit\n")
		return false
	}
	gateResults, ok := o.runComposedGates(ctx, cycle, cs, worktree, "Orchestrator.scopedMergeCarryForward", "scoped merge review")
	if !ok {
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
		TreeStateSHA: worktreeContentSHA(ctx, projectRoot, worktree, cs.WorkspacePath),
		GateResults:  gateResults,
		AuditedDiff:  snap.Diff,
		ComposedDiff: resolution,
	}
	ledgerPath := filepath.Join(projectRoot, ".evolve", "ledger.jsonl")
	if err := o.compositionVerdictWriter(ledgerPath, in); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: writer failed (fail-closed): %v; falling back to full re-audit\n", err)
		return false
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] scoped merge review: wrote scoped-review composition-verdict for cycle %d; skipping re-audit\n", cycle)
	return true
}

func compositionPatchID(diff []byte) (string, error) {
	cmd := sysexec.Command(context.Background(), "git", "patch-id", "--stable")
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
