package core

// retry_adjudicator_agent_test.go — the parse contract. Every malformed shape must
// yield nil, because nil means "use the policy default" and that is the property
// keeping this agent an enhancement rather than a precondition.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAdjudication(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		wantNil    bool
		wantAction retryAction
	}{
		{
			name:       "well-formed proposal",
			stdout:     `{"action":"retry@build","reentry_phase":"build","justification":"tests are right; the change is wrong"}`,
			wantAction: retryActionRetryBuild,
		},
		{
			name:       "prose around the JSON is tolerated",
			stdout:     "Here is my analysis.\n{\"action\":\"decline\",\"justification\":\"a rebuild re-earns this\"}\nDone.",
			wantAction: retryActionDecline,
		},
		{name: "empty stdout and no artifact", stdout: "", wantNil: true},
		{name: "no JSON at all", stdout: "I think we should try again", wantNil: true},
		{name: "malformed JSON", stdout: `{"action": "retry@tdd",,,}`, wantNil: true},
		{name: "empty action", stdout: `{"action":"","justification":"unsure"}`, wantNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAdjudication(tc.stdout, filepath.Join(t.TempDir(), "absent.json"))

			if tc.wantNil {
				if got != nil {
					t.Errorf("got %+v, want nil — a malformed proposal must degrade to the policy default", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want a parsed proposal")
			}
			if got.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tc.wantAction)
			}
		})
	}
}

// stdout is preferred, but the artifact on disk is the fallback — the agent may
// write its answer to the contracted path rather than echoing it.
func TestParseAdjudication_FallsBackToTheArtifact(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "failure-adjudication.json")
	if err := os.WriteFile(p, []byte(`{"action":"retry@tdd","justification":"encode the defects as tests"}`), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	got := parseAdjudication("", p)

	if got == nil || got.Action != retryActionRetryTDD {
		t.Fatalf("got %+v, want the artifact's proposal", got)
	}
}

// A nil bridge must be a no-op, not a panic: the composition root may wire this
// adjudicator before a bridge exists.
func TestBridgeRetryAdjudicator_NilBridgeIsANoOp(t *testing.T) {
	a := NewBridgeRetryAdjudicator(nil, AgentIdentity{}, t.TempDir())

	if got := a.Adjudicate(CycleState{WorkspacePath: t.TempDir()}, retryEnvelope{}); got != nil {
		t.Errorf("got %+v, want nil from a bridgeless adjudicator", got)
	}
}

// The persona/profile/artifact TRIPLE must agree. Getting this wrong is the drift
// class this whole redesign kept uncovering — a gate looking for `failure-lesson*`
// while the persona wrote `inst-L*`, a config knob reaching no composition root —
// and it caught me here too: the profile was first named `failure-adjudication`
// against a persona named `failure-adjudicator`, which the repo's pairing guard
// correctly reported would die exit=10 at launch.
//
// Only the profile PATH is checkable from this package; the persona↔profile pairing
// itself is enforced by phasecoherence.TestRepoPersonaProfilePairing.
func TestAdjudicatorProfilePathMatchesTheShippedProfile(t *testing.T) {
	root := repoRootForTest(t)
	a := &bridgeRetryAdjudicator{root: root}

	if _, err := os.Stat(a.profilePath()); err != nil {
		t.Errorf("adjudicator dispatches with profile %q which does not exist (%v) — dispatch would die at launch",
			a.profilePath(), err)
	}
}

// WithRetryAdjudicator must reach the field, and the production adjudicator must
// satisfy the RetryAdjudicator interface. Both are exported surface, so both need
// a named test — the apicover Phase-5 gate enforces that, and it has caught an
// unwired export in this subsystem before.
func TestWithRetryAdjudicator(t *testing.T) {
	var _ RetryAdjudicator = (*bridgeRetryAdjudicator)(nil) // compile-time contract
	want := NewBridgeRetryAdjudicator(nil, AgentIdentity{}, "")

	o := NewOrchestrator(nil, nil, nil, WithRetryAdjudicator(want))

	if o.retryAdjudicator == nil {
		t.Fatal("WithRetryAdjudicator did not reach the field; the adjudicator would never be consulted")
	}
	// And the zero-option default must be nil — policy alone decides until an
	// adjudicator is explicitly injected.
	if plain := NewOrchestrator(nil, nil, nil); plain.retryAdjudicator != nil {
		t.Error("an un-optioned orchestrator must have no adjudicator; judgment is opt-in")
	}
}

// The reason lastBalancedSpan is used instead of a naive first-'{'/last-'}' slice:
// an artifact carrying reasoning prose plus the answer, or any stray trailing
// brace, must still yield the FINAL object rather than discarding a valid answer.
func TestParseAdjudication_RecoversTheLastObjectAmongSeveral(t *testing.T) {
	raw := `{"note":"my first draft, ignore"}` + "\n" +
		`Some reasoning about the failure. Consider {this} aside.` + "\n" +
		`{"action":"retry@build","justification":"the change is wrong, the tests are right"}`

	got := parseAdjudication(raw, filepath.Join(t.TempDir(), "absent.json"))

	if got == nil {
		t.Fatal("got nil; a naive first-brace/last-brace span would swallow all three objects and discard a valid answer")
	}
	if got.Action != retryActionRetryBuild {
		t.Errorf("Action = %q, want the LAST object's action", got.Action)
	}
}
