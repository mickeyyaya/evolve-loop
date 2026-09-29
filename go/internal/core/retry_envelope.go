package core

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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

	retryActionReauthorExplanation retryAction = "retry@explanation"
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
	if in.DeterministicFloorCandidate != "" {
		return retryEnvelope{
			Halt:   true,
			Reason: "deterministic floor candidate " + in.DeterministicFloorCandidate,
		}
	}

	class := failurelog.NormalizeLegacy(in.DeclaredClass)
	if class == failurelog.UnknownClassification {
		if strings.TrimSpace(in.DeclaredClass) == "" {
			return declineOnly("audit declared no failure class; nothing to base a retry on")
		}
		return declineOnly("audit declared a class outside the vocabulary: " + in.DeclaredClass)
	}
	declared := policyCategoryFor(class)
	if declared == "" {
		return declineOnly("no retry policy row for declared class " + string(class) + " — it names a failure no rebuild repairs")
	}
	cat, known := in.Policy.RetryPolicyFor(declared)
	if !known {
		return declineOnly("policy table has no row for category " + declared + " (declared class " + string(class) + ")")
	}
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
// action is legal — Core Agent Rule 5 (LLM cycles for qualitative work; the
// rest is deterministic code).
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

// clampAdjudication binds a proposal to the envelope, returning the action to
// take and whether the proposal had to be overridden. A nil, unjustified, or
// out-of-vocabulary proposal yields the policy default (Null Object) rather
// than "no decision". The clamp is one-directional by construction: an
// adjudicator may always choose a MORE conservative action within the
// envelope, and can never reach outside it.
func clampAdjudication(env retryEnvelope, adj *adjudication) (retryAction, bool) {
	fallback := defaultAction(env)
	if adj == nil {
		return fallback, false // absence is not a clamp; nothing was overridden
	}
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

func policyCategoryFor(c failurelog.Classification) string {
	switch c {
	case failurelog.InfrastructureSystemic:
		return policy.CategoryInfraSystemic
	case failurelog.ExitTransportHang:
		return policy.CategoryTransportHang
	case failurelog.CodeBuildFail:
		return policy.CategoryCodeBuildFail
	case failurelog.CodeAuditFail, failurelog.CodeAuditWarn:
		return policy.CategoryCodeAuditFail
	case failurelog.IntentMalformed:
		return policy.CategoryIntentMalformed
	}
	return ""
}
