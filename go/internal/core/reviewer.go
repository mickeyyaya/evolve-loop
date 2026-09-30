package core

import "context"

// ReviewInput is the bundle a DeliverableReviewer needs to decide on a phase.
// Includes everything from the PhaseResponse plus the phase identity and the
// resolved git-evidence challenge token (when CommitEvidence >= Shadow), so
// reviewers don't have to re-discover any of these.
type ReviewInput struct {
	Cycle                           int
	RunID                           string
	ExplanationDocumentationVersion int
	// WorktreeBaseSHA is the cycle's worktree base commit — reviewers diff against it, not HEAD, since a committing builder leaves `git diff HEAD` empty until the post-review soft reset.
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
	// Demoted marks that the contract gate's breaker demoted enforce→advisory; without it a demotion is indistinguishable from a compliant deliverable.
	Demoted bool
	// Blocks is the reviewer's own consecutive-contract-block counter; 0 means the deciding reviewer keeps none.
	Blocks int
	// Remediation is an optional, gate-authored instruction for satisfying this violation; empty means the correction directive is the byte-identical default, which cannot fit every failure class (some violations are fixed only by creating a missing artifact, which the default directive's "do not change unrelated files" clause forbids).
	Remediation string
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

type mandatoryExplanationReviewer struct {
	next DeliverableReviewer
}

func withMandatoryExplanationReviewer(next DeliverableReviewer) DeliverableReviewer {
	if next == nil {
		next = noopReviewer{}
	}
	return mandatoryExplanationReviewer{next: next}
}

func (r mandatoryExplanationReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) || in.ExplanationDocumentationVersion == 0 {
		return r.next.Review(ctx, in)
	}
	if floor := NewBuildExplanationReviewer().Review(ctx, in); !floor.Approve {
		return floor
	}
	optional := r.next.Review(ctx, in)
	if !optional.Approve {
		return optional
	}
	sealed := NewExplanationLifecycleReviewer().Review(ctx, in)
	if !sealed.Approve {
		return carryDemotion(sealed, optional)
	}
	return optional
}

// ContractVerification is a breaker-neutral well-formedness verdict for one
// phase deliverable. ArtifactPath is the CONTRACTED destination — the only
// path the salvage rung may relocate to.
// See ADR-0045.
type ContractVerification struct {
	OK           bool
	ArtifactPath string
	Violations   []string // "[code] message" per violation
}

// ContractVerifier re-checks a phase's deliverable WITHOUT touching the
// contract-gate circuit breaker: the correction ladder's intermediate rung
// re-checks (salvage's verify-after-move, live-fix's post-window re-verify)
// must never increment the GLOBAL breaker in deliverable/reviewer.go — a
// multi-rung repair attempt would otherwise count several blocks for one
// flaky deliverable and silently demote the contract gate batch-wide. Only
// the ladder's FINAL outcome goes through DeliverableReviewer.Review. The
// error follows deliverable.Verify's fail-open contract: err => ambiguity
// (unknown phase) => the caller skips the rung rather than acting blind.
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

// noopReviewer is the optional-reviewer default. The core-owned explanation
// wrapper still runs around it for versioned, non-degraded Build handoffs.
type noopReviewer struct{}

func (noopReviewer) Review(_ context.Context, _ ReviewInput) ReviewResult {
	return ReviewResult{Approve: true}
}

// declaredDeliverablesGate is the marker the production contract gate
// implements (internal/deliverable.Reviewer). core cannot name that type — the
// import runs the other way — so the composition-root wiring proof asks for
// the capability instead of the type.
type declaredDeliverablesGate interface{ VerifiesDeclaredDeliverables() bool }

// DeclaredDeliverablesGateWired reports whether the reviewer chain contains a
// reviewer that verifies every agent-owed declared output (ADR-0100), looking
// through the mandatory-explanation wrapper and chain nesting. Introspection
// for composition-root wiring tests (mirrors ThroughputRecorderWired).
func (o *Orchestrator) DeclaredDeliverablesGateWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		g, ok := r.(declaredDeliverablesGate)
		return ok && g.VerifiesDeclaredDeliverables()
	})
}

func reviewerChainHas(r DeliverableReviewer, pred func(DeliverableReviewer) bool) bool {
	switch v := r.(type) {
	case nil:
		return false
	case mandatoryExplanationReviewer:
		return reviewerChainHas(v.next, pred)
	case chainReviewer:
		for _, m := range v {
			if reviewerChainHas(m, pred) {
				return true
			}
		}
		return false
	default:
		return pred(r)
	}
}

// contractGateSignals is the capability the composition-root wiring proof
// asks for beside declaredDeliverablesGate: the contract gate reports its
// decisions through the Signal Center.
// See ADR-0101.
type contractGateSignals interface {
	VerifiesDeclaredDeliverables() bool
	SignalsWired() bool
}

// ContractGateSignalsWired reports whether the reviewer chain's contract gate
// was handed a Center — without it the gate's verdicts never reach the stream
// and the orchestrator's listener cannot see "checked → advanced".
func (o *Orchestrator) ContractGateSignalsWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		g, ok := r.(contractGateSignals)
		return ok && g.VerifiesDeclaredDeliverables() && g.SignalsWired()
	})
}
