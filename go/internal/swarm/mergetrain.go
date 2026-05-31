package swarm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrMergeConflict is returned by a GitMerger when a merge could not be applied
// cleanly (the merger has already aborted, leaving the integration branch at its
// prior tip).
var ErrMergeConflict = errors.New("merge conflict")

// GitMerger merges one worker dev branch into the integration branch. Injected
// so the merge-train is testable without real git. The production impl
// (ExecGitMerger) shells out; on conflict it `git merge --abort`s so the
// integration branch is never left half-merged.
type GitMerger interface {
	Merge(ctx context.Context, projectRoot, integrationBranch, fromBranch string) error
}

// AcceptanceChecker runs a worker's acceptance gate (e.g. `go test`) against the
// integration worktree AFTER its branch is merged. nil error = pass. This is the
// "gate each merge step on the acceptance check, not just git success" rule
// (research: ~80% fewer broken integrations) — a merge that text-merges but
// breaks the build is rolled back. A nil AcceptanceChecker skips the gate.
type AcceptanceChecker func(ctx context.Context, workerID, integrationBranch string) error

// ConflictResolver re-invokes the authoring worker to resolve a merge conflict
// (or acceptance failure) against the current integration tip — the
// "authoring-worker resolves its own conflicts" rule. Returns nil if resolved
// (the train then retries the merge once). A nil ConflictResolver means "no
// resolution attempt" → the step fails.
type ConflictResolver func(ctx context.Context, workerID, integrationBranch string) error

// MergeOutcome records one worker's merge-train step.
type MergeOutcome struct {
	WorkerID string
	Merged   bool
	Resolved bool   // a conflict/acceptance failure was fixed on retry
	Reason   string // failure reason when !Merged
}

// MergeReport is the whole merge-train result.
type MergeReport struct {
	Outcomes []MergeOutcome
	// AllMerged is true iff every worker landed on the integration branch.
	AllMerged bool
}

// MergeTrainDeps are the injected seams for RunMergeTrain.
type MergeTrainDeps struct {
	Merger     GitMerger
	Accept     AcceptanceChecker // optional; nil skips acceptance gating
	Resolver   ConflictResolver  // optional; nil = no conflict re-dispatch (fail on conflict)
	MaxRetries int               // conflict re-dispatch attempts per worker (default 1)
}

// RunMergeTrain serializes the worker dev-branch → integration-branch merges in
// the given topological order (from Validate/TopoOrder), gating each on its
// acceptance check. This is the WRITER fan-in. It is strictly sequential — only
// one merge touches the shared integration index at a time, so there is no
// .git/index.lock contention.
//
// Per worker: merge → (acceptance) → on conflict OR acceptance failure, invoke
// the ConflictResolver (authoring worker) up to MaxRetries and retry → still
// failing ⇒ record the failure and STOP (a half-built integration must not
// proceed; the caller falls back / fails the phase).
func RunMergeTrain(ctx context.Context, projectRoot, integrationBranch string, order []string, branchByID map[string]string, deps MergeTrainDeps) MergeReport {
	maxRetries := deps.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	} else if maxRetries == 0 {
		maxRetries = 1 // default: one authoring-worker resolution attempt
	}

	var rep MergeReport
	rep.AllMerged = len(order) > 0
	for _, id := range order {
		out := mergeOneWorker(ctx, projectRoot, integrationBranch, id, branchByID[id], deps, maxRetries)
		rep.Outcomes = append(rep.Outcomes, out)
		if !out.Merged {
			rep.AllMerged = false
			break // do not continue a half-built integration
		}
	}
	return rep
}

// mergeOneWorker runs one worker's merge + acceptance, with bounded
// conflict-resolution retries.
func mergeOneWorker(ctx context.Context, projectRoot, integ, id, branch string, deps MergeTrainDeps, maxRetries int) MergeOutcome {
	out := MergeOutcome{WorkerID: id}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		stepErr := deps.Merger.Merge(ctx, projectRoot, integ, branch)
		if stepErr == nil {
			if acErr := runAcceptance(ctx, deps.Accept, id, integ); acErr == nil {
				out.Merged = true
				out.Resolved = attempt > 0
				return out
			} else {
				stepErr = fmt.Errorf("acceptance: %w", acErr)
			}
		}
		// Failed (conflict or acceptance). Try the authoring worker once more.
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

// ExecGitMerger is the production GitMerger. It merges fromBranch into the
// integration branch (whose worktree is IntegrationWorktree) with a merge
// commit; on conflict it aborts so the integration tip is unchanged.
type ExecGitMerger struct {
	// IntegrationWorktree is the path whose checked-out branch is the integration
	// branch (merges run with -C here so the shared index is the integration one).
	IntegrationWorktree string
}

// Merge implements GitMerger.
func (m ExecGitMerger) Merge(ctx context.Context, _, _, fromBranch string) error {
	dir := m.IntegrationWorktree
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "merge", "--no-ff", "--no-edit", fromBranch)
	var eb bytes.Buffer
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		// Abort so the integration branch is left clean for the next attempt.
		_ = exec.CommandContext(ctx, "git", "-C", dir, "merge", "--abort").Run()
		return fmt.Errorf("%w: merge %s: %v: %s", ErrMergeConflict, fromBranch, err, eb.String())
	}
	return nil
}

// Synthesize is the READER fan-in: it concatenates the workers' summary
// artifacts (in the given order) into one merged document. Readers do no git
// merge — overlap is harmless, so synthesis simply joins the parts with a
// per-worker header. The caller supplies each worker's artifact text.
func Synthesize(order []string, artifactByID map[string]string) string {
	var b bytes.Buffer
	for _, id := range order {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", id, artifactByID[id])
	}
	return b.String()
}
