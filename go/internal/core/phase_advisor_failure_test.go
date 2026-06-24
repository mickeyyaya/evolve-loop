package core

// phase_advisor_failure_test.go — failure floor Phase 3: at failure
// transitions the routing prompt teaches the advisor the failure
// vocabulary (recovery_action / learning_richness / failure-scoped
// inserts) and extends the strict-JSON schema; happy-path prompts stay
// byte-identical (prompt-prefix cache).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestBuildRoutingPrompt_FailureTransitionVocabulary(t *testing.T) {
	t.Parallel()
	p := buildRoutingPrompt(router.RouteInput{Current: "retrospective", Verdict: "FAIL"})
	for _, want := range []string{
		"recovery_action",
		"learning_richness",
		"fault-localization",
		"bug-reproduction",
		"non-overridable", // BLOCK floor stated to the model
	} {
		if !strings.Contains(p, want) {
			t.Errorf("failure-transition prompt missing %q", want)
		}
	}

	// audit FAIL is also a failure transition (richness decision point).
	pa := buildRoutingPrompt(router.RouteInput{Current: "audit", Verdict: "FAIL"})
	if !strings.Contains(pa, "learning_richness") {
		t.Error("audit-FAIL prompt must carry the failure vocabulary")
	}
}

func TestBuildRoutingPrompt_HappyPathUnchanged(t *testing.T) {
	t.Parallel()
	for _, in := range []router.RouteInput{
		{Current: "build", Verdict: "PASS"},
		{Current: "audit", Verdict: "PASS"},
	} {
		p := buildRoutingPrompt(in)
		if strings.Contains(p, "recovery_action") || strings.Contains(p, "learning_richness") {
			t.Errorf("happy-path prompt for %s must not carry failure vocabulary (cache prefix)", in.Current)
		}
	}
}

// A failure-transition proposal may carry ONLY the failure fields — that
// is not an empty proposal.
func TestParseProposal_FailureFieldsOnlyIsNotEmpty(t *testing.T) {
	t.Parallel()
	p, err := parseProposal(`{"recovery_action":"end","justification":"budget"}`)
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if p.RecoveryAction != "end" {
		t.Errorf("RecoveryAction = %q, want end", p.RecoveryAction)
	}

	if _, err := parseProposal(`{"justification":"nothing actionable"}`); err == nil {
		t.Error("a proposal with no routing content must still be rejected as empty")
	}
}
