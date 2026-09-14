package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// The audit is told the failurelog vocabulary (13 classes) while the retry
// table is keyed by the policy vocabulary (7 categories); before this join the
// envelope looked a failurelog spelling up in the policy table and declined
// "unrecognised class" for words the prompt itself recommended (architecture
// review of F19). One projection joins the two, and the decline reason says
// which of the three things happened.
func TestPolicyCategoryFor_JoinsEveryKnownClassificationOrSaysNone(t *testing.T) {
	pol := policy.DefaultSystemFailurePolicy()
	for _, c := range failurelog.KnownClassifications() {
		cat := policyCategoryFor(c)
		if cat == "" {
			continue // no repair row: the envelope declines with "no retry policy row"
		}
		if _, ok := pol.RetryPolicyFor(cat); !ok {
			t.Errorf("%s joins to policy category %q, which the default table does not know", c, cat)
		}
	}
	for c, want := range map[failurelog.Classification]string{
		failurelog.InfrastructureSystemic: policy.CategoryInfraSystemic,
		failurelog.ExitTransportHang:      policy.CategoryTransportHang,
		failurelog.CodeAuditFail:          policy.CategoryCodeAuditFail,
		failurelog.CodeAuditWarn:          policy.CategoryCodeAuditFail,
		failurelog.CodeBuildFail:          policy.CategoryCodeBuildFail,
		failurelog.IntentMalformed:        policy.CategoryIntentMalformed,
		failurelog.HumanAbort:             "",
		failurelog.UnknownClassification:  "",
	} {
		if got := policyCategoryFor(c); got != want {
			t.Errorf("policyCategoryFor(%s) = %q, want %q", c, got, want)
		}
	}
}

func TestComputeRetryEnvelope_DeclineReasonsNameWhichVocabularyFailed(t *testing.T) {
	pol := policy.DefaultSystemFailurePolicy()
	reason := func(class string) string {
		return computeRetryEnvelope(retryEnvelopeInput{DeclaredClass: class, Policy: pol}).Reason
	}
	if r := reason("superseded-predicate-contradiction"); !strings.Contains(r, "outside the vocabulary") {
		t.Errorf("an invented class: %q", r)
	}
	if r := reason("human-abort"); !strings.Contains(r, "no retry policy row") || strings.Contains(r, "unrecognised") {
		t.Errorf("a known class with no repair row is not 'unrecognised': %q", r)
	}
	if r := reason("infrastructure-systemic"); !strings.Contains(r, "system-level") || strings.Contains(r, "unrecognised") {
		t.Errorf("a known system-level class declines as system-level, not 'unrecognised': %q", r)
	}
	if env := computeRetryEnvelope(retryEnvelopeInput{DeclaredClass: "code-audit-warn", Policy: pol}); env.Halt || len(env.Legal) < 2 {
		t.Errorf("code-audit-warn joins the task-level audit row and may retry: %+v", env)
	}
}
