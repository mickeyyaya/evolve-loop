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
	// WorktreeBaseSHA is the cycle's worktree base commit (CycleState
	// .WorktreeBaseSHA) — deterministic reviewers diff against IT, not HEAD,
	// because a committing builder (the mandated protocol) makes `git diff
	// HEAD` empty until the post-review soft-reset re-exposes the work.
	WorktreeBaseSHA string

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
	// Demoted marks that the contract gate's own circuit breaker gave up and
	// demoted enforce→advisory (deliverable.Reviewer's consecutive-block
	// breaker). Without this flag a demotion is structurally identical to a
	// compliant deliverable, so the orchestrator cannot report that a gate
	// stopped being enforced — the batch-19/batch-21 blind spot (inbox
	// contract-block-cli-escalation). Reason carries the last violation.
	//
	// Normally paired with Approve=true (the demotion IS the approval), but
	// ChainReviewers carries it onto a rejection from a LATER gate in the chain
	// too: the contract gate stopped enforcing whether or not topngate/triagecap
	// went on to reject the same deliverable.
	Demoted bool
	// Blocks is the reviewer's own count of CONSECUTIVE contract blocks including
	// this one — the breaker count that will open the circuit at its threshold.
	// 0 means the deciding reviewer keeps no such counter (evalgate, topngate,
	// triagecap, the build floor): those rejections are task-binding or capacity
	// failures, not format-compliance failures, so the orchestrator must not read
	// them as CLI evidence. The correction ladder keys its CLI escalation off
	// THIS, never off a locally re-counted correction ordinal — the two desync
	// whenever a prior cycle left the breaker hot or the salvage rung consumed a
	// block without a re-dispatch.
	Blocks int
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

// ContractVerification is a breaker-neutral well-formedness verdict for one
// phase deliverable (ADR-0045 I2). ArtifactPath is the CONTRACTED destination
// — the only path the salvage rung may relocate to.
type ContractVerification struct {
	OK           bool
	ArtifactPath string
	Violations   []string // "[code] message" per violation
}

// ContractVerifier re-checks a phase's deliverable WITHOUT touching the
// contract-gate circuit breaker. The I2 integrity rule (cycle-265 forensics):
// the correction ladder's intermediate rung re-checks (salvage's
// verify-after-move, live-fix's post-window re-verify) must never increment
// the GLOBAL breaker in deliverable/reviewer.go — a multi-rung repair attempt
// would otherwise count three blocks for one flaky deliverable and silently
// demote the contract gate batch-wide. Only the ladder's FINAL outcome goes
// through DeliverableReviewer.Review. The error follows deliverable.Verify's
// fail-open contract: err => ambiguity (unknown phase) => the caller skips
// the rung rather than acting blind.
type ContractVerifier interface {
	VerifyDeliverable(ctx context.Context, in ReviewInput) (ContractVerification, error)
}

// ChainReviewers composes reviewers into one that approves only when ALL
// approve; the first rejection short-circuits and is returned verbatim (Chain
// of Responsibility). nil entries are skipped. Used to mount the evalgate gates
// and the deliverable-contract gate (ADR-0034) at the single orchestrator seam.
func ChainReviewers(reviewers ...DeliverableReviewer) DeliverableReviewer {
	return chainReviewer(reviewers)
}

type chainReviewer []DeliverableReviewer

func (c chainReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	// Demoted must survive BOTH exits. Production mounts the contract gate in the
	// middle of this chain (cmd_cycle.go: buildFloor → evalgate → contract →
	// triagecap → topngate), so:
	//   - rebuilding a bare Approve:true would swallow the gate's own "I gave up
	//     enforcing" report on the all-approve path, and
	//   - returning a LATER reviewer's rejection verbatim would swallow it too —
	//     the case that actually bites triage, whose triagecap gate sits directly
	//     after the contract gate. Either way the orchestrator goes blind and the
	//     demotion is invisible exactly as before this fix.
	out := ReviewResult{Approve: true}
	for _, r := range c {
		if r == nil {
			continue
		}
		res := r.Review(ctx, in)
		if !res.Approve {
			return carryDemotion(res, out)
		}
		if res.Demoted && !out.Demoted {
			out.Demoted = true
			out.Reason = res.Reason
			out.Blocks = res.Blocks
		}
	}
	return out
}

// carryDemotion attaches an earlier reviewer's demotion evidence to the decision
// that short-circuits the chain, without overriding evidence the decider carries
// itself. The rejection stays verbatim in every other respect.
func carryDemotion(decision, seen ReviewResult) ReviewResult {
	if !seen.Demoted {
		return decision
	}
	decision.Demoted = true
	if decision.Reason == "" {
		decision.Reason = seen.Reason
	}
	if decision.Blocks == 0 {
		decision.Blocks = seen.Blocks
	}
	return decision
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
