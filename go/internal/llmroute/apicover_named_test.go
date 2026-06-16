package llmroute

import "testing"

// TestAutoModel_NamedSeamExpandsAuto names the llmroute.AutoModel function type
// (supplied to Resolve but never named in a test) and pins its contract: when
// the resolved model is the "auto" sentinel, Resolve invokes the seam with the
// PHASE role and substitutes the concrete model it returns.
func TestAutoModel_NamedSeamExpandsAuto(t *testing.T) {
	var gotRole string
	var expand AutoModel = func(role string) (string, bool) {
		gotRole = role
		return "claude-opus-4-7", true
	}

	got := Resolve("auditor", "audit", "auto", nil, nil, expand, nil)
	if got.Model != "claude-opus-4-7" {
		t.Errorf("Model = %q, want claude-opus-4-7 (AutoModel must expand the sentinel)", got.Model)
	}
	if gotRole != "audit" {
		t.Errorf("AutoModel invoked with role = %q, want audit (the phase, not the agent)", gotRole)
	}
}
