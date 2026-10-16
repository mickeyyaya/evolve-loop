package core

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type retryAction string

const (
	retryActionRetryTDD   retryAction = "retry@tdd"
	retryActionRetryBuild retryAction = "retry@build"
	retryActionDecline    retryAction = "decline"

	retryActionReauthorExplanation retryAction = "retry@explanation"
)

type retryEnvelopeInput struct {
	DeterministicFloorCandidate string
	DeclaredClass               string
	Attempts                    int
	Policy                      policy.SystemFailurePolicy
}

type retryEnvelope struct {
	Legal  []retryAction
	Halt   bool
	Reason string
}

func declineOnly(reason string) retryEnvelope {
	return retryEnvelope{Legal: []retryAction{retryActionDecline}, Reason: reason}
}

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

func narrowRetryEnvelope(env retryEnvelope, reentry retryAction, why string) retryEnvelope {
	if !slices.Contains(env.Legal, retryActionRetryBuild) {
		return env
	}
	return retryEnvelope{Legal: []retryAction{reentry, retryActionDecline}, Reason: env.Reason + "; " + why}
}

type adjudication struct {
	Action        retryAction
	ReentryPhase  string
	Justification string
}

func adjudicationNeeded(env retryEnvelope) bool {
	return !env.Halt && len(env.Legal) > 1
}

func defaultAction(env retryEnvelope) retryAction {
	if env.Halt || len(env.Legal) == 0 {
		return retryActionDecline
	}
	return env.Legal[0]
}

func clampAdjudication(env retryEnvelope, adj *adjudication) (retryAction, bool) {
	fallback := defaultAction(env)
	if adj == nil {
		return fallback, false
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
