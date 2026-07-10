package phasestream

// RED regression test for cycle-654 top_n task `infra-classifier-echo-veto`
// at the NORMALIZER emit layer (lesson cycle-641 preventiveAction #1 + #4):
// the incident classifier MUST NOT emit an infra_failure INCIDENT for a line
// that is a verbatim echo of the injected prompt/instruction text; a genuine
// runtime line (not present in the prompt) MUST still emit.
//
// This exercises the real Classifier.Stderr emit path and asserts on the
// emitted envelopes' Kind — not a source-grep. RED today: the Classifier has
// no injected-prompt context (SetInjectedPrompt absent → compile failure), so
// an echoed "missing rate limits." matches the rate_limit marker regex and
// emits infra_failure. The fix threads the injected prompt into the Classifier
// (SetInjectedPrompt) and drops any marker hit whose line is a substring of it.
//
// Ported (renamed C653→C654) from the fix-of-record RED suite preserved in
// .evolve/worktrees/cycle-21f9f7ae-653; do not author duplicate C653 copies.

import "testing"

func hasInfraFailure(envs []Envelope) bool {
	for _, e := range envs {
		if e.Kind == KindInfraFailure {
			return true
		}
	}
	return false
}

// TestC654_003_PromptEchoNotEmittedGenuineEmitted — normalizer prompt-echo
// exclusion. The echoed reviewer checklist line ("...missing rate limits.") is a
// substring of the injected prompt and MUST be suppressed; a genuine provider
// error frame ("429 Too Many Requests"), absent from the prompt, MUST still emit.
func TestC654_003_PromptEchoNotEmittedGenuineEmitted(t *testing.T) {
	const prompt = "Adversarial Reviewer checklist: unbounded allocation or recursion; " +
		"TOCTOU / race windows; missing rate limits. Report exploits only."

	c := NewClassifier(Source{Producer: "normalizer", Phase: "adversarial-review"}, "trace-654", nil)
	c.SetInjectedPrompt(prompt)

	// Echoed prompt line — matches the rate_limit marker but is verbatim prompt text.
	if hasInfraFailure(c.Stderr([]byte("missing rate limits."))) {
		t.Errorf("normalizer emitted infra_failure for a line that is a verbatim echo of the injected prompt")
	}

	// Genuine runtime error frame — NOT present in the prompt — must still surface.
	if !hasInfraFailure(c.Stderr([]byte("Error: 429 Too Many Requests (rate limit hit)"))) {
		t.Errorf("normalizer suppressed a genuine runtime infra signal that is absent from the prompt")
	}
}
