package core

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// retry_envelope.go — what the DETERMINISTIC policy declares legal after an audit
// FAIL. A Specification: a pure predicate, no I/O, so the same answer comes out on
// the live, routed and resume paths and one table can pin every branch.
//
// It makes ADR-0072's category table the single retry authority. That table has
// always declared
//
//	CategoryCodeAuditFail: {Level: LevelTask, Action: ActionRetryWithFix,
//	                        FixType: "address-audit-findings", MaxRetries: 2}
//
// while `fp.Categories[...]` was read in exactly one place, only for Level —
// Action, MaxRetries and FixType were consumed nowhere. ADR-0092 then built a
// parallel knob and a disposition-prose eligibility rule beside it, arriving at
// the same cap of 2 independently. One authority now, not two.
//
// The envelope only says what is LEGAL. Choosing among legal actions is the
// adjudicator's job, and it can never widen this set.

// retryAction is the closed vocabulary of dispositions after an audit FAIL.
// Declared as consts rather than string literals at call sites: a literal that
// drifts from its reader is exactly how the retro gate came to look for
// `failure-lesson*.yaml` while the persona wrote `inst-L*.yaml`.
type retryAction string

const (
	// retryActionRetryTDD re-enters the dev cycle at the test-first phase, so the
	// audit's defects are encoded as failing tests before the rebuild.
	retryActionRetryTDD retryAction = "retry@tdd"
	// retryActionRetryBuild re-enters at build — cheaper, and correct when the
	// defect is in the change rather than in what the tests assert.
	retryActionRetryBuild retryAction = "retry@build"
	// retryActionDecline ends the cycle through the terminal retro. It is always
	// legal: declining to retry is never unsafe.
	retryActionDecline retryAction = "decline"
)

// retryEnvelopeInput inverts every I/O concern out of the rule. The policy is
// PASSED so tests can drive real category tables rather than fixtures of them,
// and so the rule can never read a different policy than its caller enforced.
type retryEnvelopeInput struct {
	// DeterministicFloorCandidate is failureDossier.FloorCandidate. Non-empty ⇒
	// halt, evaluated before any policy lookup (ADR-0072 gate 1 is absolute).
	DeterministicFloorCandidate string
	// DeclaredClass is the audit's OWN failure class, read from its report
	// sentinel via phasecontract.ReadFailureBlock — machine-readable, written by
	// the auditor, and not a paraphrase of it.
	DeclaredClass string
	// Attempts is how many retries this cycle has already spent.
	Attempts int
	// Policy is the resolved ADR-0072 failure policy.
	Policy policy.SystemFailurePolicy
}

// retryEnvelope is the legal action set plus the halt decision. Reason is always
// populated: a disposition an operator cannot read is one they cannot audit.
type retryEnvelope struct {
	Legal  []retryAction
	Halt   bool
	Reason string
}

// declineOnly is the conservative envelope — used wherever evidence is absent or
// unrecognised. Absence of evidence never grants a retry, and never halts the
// loop either: an unknown class is a reason to stop this cycle, not the batch.
func declineOnly(reason string) retryEnvelope {
	return retryEnvelope{Legal: []retryAction{retryActionDecline}, Reason: reason}
}

// computeRetryEnvelope applies the deterministic policy.
func computeRetryEnvelope(in retryEnvelopeInput) retryEnvelope {
	// Gate 1 parity: a deterministic floor candidate ends the conversation before
	// any class lookup, so a broken pipeline cannot buy a retry by declaring a
	// friendly class.
	if in.DeterministicFloorCandidate != "" {
		return retryEnvelope{
			Halt:   true,
			Reason: "deterministic floor candidate " + in.DeterministicFloorCandidate,
		}
	}

	// Normalize before lookup: the audit writes a free-form class string, and the
	// repo runs TWO vocabularies — policy uses infra-systemic/transport-hang while
	// failurelog uses infrastructure-systemic/exit-transport-hang, and legacy
	// "audit-fail"/"FAIL" both mean code-audit-fail. Every sibling consumer
	// normalizes; skipping it here would silently decline a retryable class
	// (architect review M4).
	declared := string(failurelog.NormalizeLegacy(in.DeclaredClass))

	cat, known := in.Policy.RetryPolicyFor(declared)
	if !known {
		if declared == "" {
			return declineOnly("audit declared no failure class; nothing to base a retry on")
		}
		return declineOnly("audit declared an unrecognised class " + declared)
	}
	// NOTE: a system-level declared class does NOT halt here (architect review H4).
	// Both pre-existing floor gates require IsFloor, and gate 2 was deliberately
	// narrowed to lose against a contradicting disposition. Minting a new halt from
	// an agent-written class — with no floor check and no corroboration — would
	// reintroduce the very disease this redesign cures, and would let non-floor
	// system categories (transport-hang, non-progress) stop the batch on prose.
	// Declining routes to retro, where the full two-gate floor decides. Only the
	// DETERMINISTIC candidate halts at this chokepoint.
	if cat.Level == policy.LevelSystem {
		return declineOnly("declared class " + declared + " is system-level; the retro floor gates adjudicate it")
	}
	if cat.Action != policy.ActionRetryWithFix {
		return declineOnly("declared class " + declared + " maps to " + string(cat.Action) + ", not a retry")
	}
	if in.Attempts >= cat.MaxRetries {
		return declineOnly("retry budget spent for " + declared +
			" (" + strconv.Itoa(in.Attempts) + "/" + strconv.Itoa(cat.MaxRetries) + ")")
	}

	return retryEnvelope{
		Legal: []retryAction{retryActionRetryTDD, retryActionRetryBuild, retryActionDecline},
		Reason: "policy " + declared + " ⇒ " + string(cat.Action) + " (" + cat.FixType + "), attempt " +
			strconv.Itoa(in.Attempts+1) + "/" + strconv.Itoa(cat.MaxRetries),
	}
}

// adjudication is a deep-tier phase's PROPOSAL for how to dispose of an audit
// FAIL. It is deliberately a proposal and not a decision: clampAdjudication binds
// it to the deterministic envelope, so the phase chooses among legal options and
// can never create one.
type adjudication struct {
	Action        retryAction
	ReentryPhase  string
	Justification string
}

// adjudicationNeeded reports whether there is genuinely a choice to make.
// Judgment costs a deep-tier dispatch, so it is paid only where more than one
// action is legal — Core Agent Rule 5 (LLM cycles for qualitative work; the rest
// is deterministic code) and the tiered-detector pattern from the 2026 literature.
func adjudicationNeeded(env retryEnvelope) bool {
	return !env.Halt && len(env.Legal) > 1
}

// defaultAction is the envelope's own preference. Legal is ORDERED most-thorough
// first, so the default needs no second table that could drift from the first.
// An empty or halting envelope declines: never nothing, always something safe.
func defaultAction(env retryEnvelope) retryAction {
	if env.Halt || len(env.Legal) == 0 {
		return retryActionDecline
	}
	return env.Legal[0]
}

// clampAdjudication binds a proposal to the envelope, returning the action to take
// and whether the proposal had to be overridden.
//
// NULL OBJECT: a nil, unjustified, or out-of-vocabulary proposal yields the policy
// default rather than "no decision". No agent artifact is load-bearing here — the
// ADR-0092 failure (retry reachable on 3 of 16 cycles because the artifact was
// usually missing) is designed out rather than mitigated.
//
// The clamp is one-directional by construction: an adjudicator may always choose a
// MORE conservative action within the envelope, and can never reach outside it.
func clampAdjudication(env retryEnvelope, adj *adjudication) (retryAction, bool) {
	fallback := defaultAction(env)
	if adj == nil {
		return fallback, false // absence is not a clamp; nothing was overridden
	}
	// An unjustified proposal is not usable input: this phase exists for its
	// reasoning, not for its verdict word.
	if strings.TrimSpace(adj.Justification) == "" {
		return fallback, true
	}
	for _, legal := range env.Legal {
		if adj.Action == legal {
			return adj.Action, false
		}
	}
	return fallback, true
}
