package swarm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// ErrMergeConflict wraps a merge that could not be applied; the merger has already aborted it.
var ErrMergeConflict = errors.New("merge conflict")

// GitMerger merges one worker dev branch into the integration branch and never leaves it half-merged.
type GitMerger interface {
	Merge(ctx context.Context, integrationBranch, fromBranch string) error
}

// AcceptanceChecker runs a worker's acceptance check against the integration tip after its merge; nil error passes.
type AcceptanceChecker func(ctx context.Context, workerID, integrationBranch string) error

// ConflictResolver re-invokes the authoring worker to fix a failed merge step; nil error means retry the merge.
type ConflictResolver func(ctx context.Context, workerID, integrationBranch string) error

// MergeOutcome records one worker's merge-train step.
type MergeOutcome struct {
	WorkerID string
	Merged   bool
	Resolved bool   // a conflict or acceptance failure was fixed on retry
	Reason   string // failure reason when !Merged
}

// MergeReport is the whole merge-train result.
type MergeReport struct {
	Outcomes  []MergeOutcome
	AllMerged bool // every worker landed; false for an empty order
}

// MergeTrainDeps are the injected seams for RunMergeTrain.
type MergeTrainDeps struct {
	Merger     GitMerger
	Accept     AcceptanceChecker // nil skips acceptance gating
	Resolver   ConflictResolver  // nil fails a step on its first conflict
	MaxRetries int               // resolution attempts per worker: 0 means 1, negative means none
}

// RunMergeTrain merges workers one at a time in the given order, gating each on acceptance, and stops at the first failure.
func RunMergeTrain(ctx context.Context, integrationBranch string, order []string, branchByID map[string]string, deps MergeTrainDeps) MergeReport {
	maxRetries := deps.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	} else if maxRetries == 0 {
		maxRetries = 1
	}

	var rep MergeReport
	rep.AllMerged = len(order) > 0
	for _, id := range order {
		out := mergeOneWorker(ctx, integrationBranch, id, branchByID[id], deps, maxRetries)
		rep.Outcomes = append(rep.Outcomes, out)
		if !out.Merged {
			rep.AllMerged = false
			break // never build on a half-built integration
		}
	}
	return rep
}

func mergeOneWorker(ctx context.Context, integ, id, branch string, deps MergeTrainDeps, maxRetries int) MergeOutcome {
	out := MergeOutcome{WorkerID: id}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		stepErr := deps.Merger.Merge(ctx, integ, branch)
		if stepErr == nil {
			if acErr := runAcceptance(ctx, deps.Accept, id, integ); acErr == nil {
				out.Merged = true
				out.Resolved = attempt > 0
				return out
			} else {
				stepErr = fmt.Errorf("acceptance: %w", acErr)
			}
		}
		out.Reason = stepErr.Error()
		if attempt == maxRetries || deps.Resolver == nil {
			return out
		}
		if rErr := deps.Resolver(ctx, id, integ); rErr != nil {
			out.Reason = fmt.Sprintf("conflict resolution failed: %v (after %v)", rErr, stepErr)
			return out
		}
	}
	return out
}

func runAcceptance(ctx context.Context, ac AcceptanceChecker, id, integ string) error {
	if ac == nil {
		return nil
	}
	return ac(ctx, id, integ)
}

// ExecGitMerger is the production GitMerger: a --no-ff merge run inside the integration worktree.
type ExecGitMerger struct {
	// IntegrationWorktree has the integration branch checked out, so merges use its index.
	IntegrationWorktree string
}

// Merge implements GitMerger.
func (m ExecGitMerger) Merge(ctx context.Context, _ /*integrationBranch*/, fromBranch string) error {
	return mergeWith(ctx, gitexec.Default(m.IntegrationWorktree), fromBranch)
}

func mergeWith(ctx context.Context, g gitexec.Git, fromBranch string) error {
	_, stderr, code, err := g.Capture(ctx, "merge", "--no-ff", "--no-edit", fromBranch)
	if err != nil || code != 0 {
		// Abort so the integration branch stays at its prior tip for the next attempt.
		_ = g.Run(ctx, "merge", "--abort")
		return fmt.Errorf("%w: merge %s: %s: %s", ErrMergeConflict, fromBranch, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
	return nil
}

// Synthesize is the reader fan-in: it joins worker artifacts in order, each under a per-worker header.
func Synthesize(order []string, artifactByID map[string]string) string {
	var b bytes.Buffer
	for _, id := range order {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", id, artifactByID[id])
	}
	return b.String()
}
