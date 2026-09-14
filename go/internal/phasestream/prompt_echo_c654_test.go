package phasestream

import "testing"

func hasInfraFailure(envs []Envelope) bool {
	for _, e := range envs {
		if e.Kind == KindInfraFailure {
			return true
		}
	}
	return false
}

// Preserve both the API contract and cycle-654's infra-classifier-echo-veto
// incident input (the cycle-641/642 reviewer-checklist regression). The direct
// SetInjectedPrompt call also binds its exported identifier for apicover.
func TestClassifier_SetInjectedPrompt(t *testing.T) {
	for _, tc := range []struct{ name, prompt, trace string }{
		{"api_contract", "Adversarial Reviewer checklist: TOCTOU / race windows; missing rate limits.", "trace-cover"},
		{"cycle654_incident", "Adversarial Reviewer checklist: unbounded allocation or recursion; " +
			"TOCTOU / race windows; missing rate limits. Report exploits only.", "trace-654"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClassifier(Source{Producer: "normalizer", Phase: "adversarial-review"}, tc.trace, nil)
			c.SetInjectedPrompt(tc.prompt)

			if hasInfraFailure(c.Stderr([]byte("missing rate limits."))) {
				t.Error("normalizer emitted infra_failure for a verbatim echo of the injected prompt")
			}
			if !hasInfraFailure(c.Stderr([]byte("Error: 429 Too Many Requests (rate limit hit)"))) {
				t.Error("normalizer suppressed a genuine runtime infra signal absent from the prompt")
			}
		})
	}
}
