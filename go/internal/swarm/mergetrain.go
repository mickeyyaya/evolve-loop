package swarm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

var ErrMergeConflict = errors.New("merge conflict")

type GitMerger interface {
	Merge(ctx context.Context, integrationBranch, fromBranch string) error
}

type AcceptanceChecker func(ctx context.Context, workerID, integrationBranch string) error

type ConflictResolver func(ctx context.Context, workerID, integrationBranch string) error

type MergeOutcome struct {
	WorkerID string
	Merged   bool
	Resolved bool
	Reason   string
}

type MergeReport struct {
	Outcomes  []MergeOutcome
	AllMerged bool
}

type MergeTrainDeps struct {
	Merger     GitMerger
	Accept     AcceptanceChecker
	Resolver   ConflictResolver
	MaxRetries int
}

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
			break
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

type ExecGitMerger struct {
	IntegrationWorktree string
}

func (m ExecGitMerger) Merge(ctx context.Context, _, fromBranch string) error {
	return mergeWith(ctx, gitexec.Default(m.IntegrationWorktree), fromBranch)
}

func mergeWith(ctx context.Context, g gitexec.Git, fromBranch string) error {
	_, stderr, code, err := g.Capture(ctx, "merge", "--no-ff", "--no-edit", fromBranch)
	if err != nil || code != 0 {
		_ = g.Run(ctx, "merge", "--abort")
		return fmt.Errorf("%w: merge %s: %s: %s", ErrMergeConflict, fromBranch, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
	return nil
}

func Synthesize(order []string, artifactByID map[string]string) string {
	var b bytes.Buffer
	for _, id := range order {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", id, artifactByID[id])
	}
	return b.String()
}
