package llmroute

import (
	"testing"
	"time"
)

// TestApplyDriverBench_NamingAndDemote names ApplyDriverBench and pins its
// contract: a driver-benched candidate is moved to the end of the chain while
// healthy drivers stay first (driver-scoped, not Family()-scoped).
func TestApplyDriverBench_NamingAndDemote(t *testing.T) {
	p := Plan{Candidates: []string{"codex-tmux", "claude-tmux"}, Triggers: []int{80}}
	benched := map[string]time.Time{"codex-tmux": time.Now()}
	got := ApplyDriverBench(p, benched)
	if len(got.Candidates) != 2 {
		t.Fatalf("ApplyDriverBench: len(Candidates)=%d, want 2", len(got.Candidates))
	}
	if got.Candidates[0] != "claude-tmux" {
		t.Errorf("ApplyDriverBench: Candidates[0]=%q, want claude-tmux (healthy first)", got.Candidates[0])
	}
	if got.Candidates[1] != "codex-tmux" {
		t.Errorf("ApplyDriverBench: Candidates[1]=%q, want codex-tmux (benched demoted)", got.Candidates[1])
	}
	// "codex" (family-headless) must NOT be demoted — driver-scoped only.
	p2 := Plan{Candidates: []string{"codex", "claude-tmux"}, Triggers: []int{80}}
	got2 := ApplyDriverBench(p2, benched)
	if got2.Candidates[0] != "codex" {
		t.Errorf("ApplyDriverBench: \"codex\" demoted by bench of \"codex-tmux\" — must be driver-scoped not family-scoped")
	}
}

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
