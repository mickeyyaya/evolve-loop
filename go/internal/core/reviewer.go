package core

// reviewer.go — Workstream E2: per-phase deliverable review gate.
//
// After runner.Run returns a non-SKIPPED verdict, the orchestrator consults a
// DeliverableReviewer BEFORE recording the phase as a success (ledger append,
// CompletedPhases++, current=next). A nil reviewer is the default and a no-op
// — byte-identical to the pre-E2 cycle when not opted in. Non-nil reviewers
// (the deterministic default registered via WithReviewer, or a future LLM
// reviewer backed by ollama-tmux at ReviewGate=enforce) may approve, reject,
// or request a retry.
//
// The interface is small on purpose: presence is the contract, not shape. The
// deterministic default reviewer + the LLM reviewer share this interface; an
// operator can swap one for the other without touching the orchestrator.

import "context"

// ReviewInput is the bundle a DeliverableReviewer needs to decide on a phase.
// Includes everything from the PhaseResponse plus the phase identity and the
// resolved git-evidence challenge token (when CommitEvidence >= Shadow), so
// reviewers don't have to re-discover any of these.
type ReviewInput struct {
	Phase          string        // phase name ("tdd", "build", ...)
	Response       PhaseResponse // the runner's PhaseResponse for the just-finished phase
	Workspace      string        // absolute workspace dir (artifacts live here)
	Worktree       string        // absolute worktree dir; "" for non-worktree (read-only) phases
	ProjectRoot    string        // absolute project root (for git-evidence verification)
	ChallengeToken string        // <workspace>/challenge-token.txt; empty if the phase didn't emit one
}

// ReviewResult is the reviewer's decision.
//
//	Approve=true  → phase is recorded as a success (cycle advances).
//	Approve=false → phase is REJECTED. The orchestrator MUST surface Reason
//	                in the cycle failure record and either retry (Retry=true,
//	                up to a per-orchestrator retry budget) or abort.
//
// Reason MUST be non-empty when Approve=false — operators need to know WHY a
// deliverable was rejected to fix the underlying issue.
type ReviewResult struct {
	Approve bool
	Reason  string
	Retry   bool
}

// DeliverableReviewer adjudicates a finished phase's deliverable. Implementations
// MUST be safe to call from the orchestrator's main loop (single-call-per-phase;
// no concurrency required). A nil reviewer means "no review" — the orchestrator
// accepts every non-error, non-SKIPPED verdict as a pass (pre-E2 behavior).
//
// The deterministic default reviewer (DefaultDeliverableReviewer) checks
// presence + shape: source-writing phases at CommitEvidence>=Shadow require a
// valid Evolve-Phase trailer + challenge-token match; other phases require
// the artifact file the runner produced. Future LLM reviewers can wrap or
// replace the default.
type DeliverableReviewer interface {
	Review(ctx context.Context, in ReviewInput) ReviewResult
}

// noopReviewer is the orchestrator's default when WithReviewer was not used:
// every phase is approved unconditionally, exactly reproducing the pre-E2
// cycle. Kept here (not in a separate file) so the contract — "nil reviewer
// implies this exact behavior" — lives next to the interface that defines it.
type noopReviewer struct{}

// Review implements DeliverableReviewer with a permissive default.
func (noopReviewer) Review(_ context.Context, _ ReviewInput) ReviewResult {
	return ReviewResult{Approve: true}
}
