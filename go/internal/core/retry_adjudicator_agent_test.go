package core

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

func TestBridgeRetryAdjudicator_NilBridgeIsANoOp(t *testing.T) {
	a := NewBridgeRetryAdjudicator(nil, AgentIdentity{}, t.TempDir())

	if got := a.Adjudicate(CycleState{WorkspacePath: t.TempDir()}, retryEnvelope{}); got != nil {
		t.Errorf("got %+v, want nil from a bridgeless adjudicator", got)
	}
}

// Only the profile path is checkable from this package; the persona↔profile
// pairing itself is enforced by phasecoherence.TestRepoPersonaProfilePairing.
func TestAdjudicatorProfilePathMatchesTheShippedProfile(t *testing.T) {
	root := repoRootForTest(t)
	a := &bridgeRetryAdjudicator{root: root}

	if _, err := os.Stat(a.profilePath()); err != nil {
		t.Errorf("adjudicator dispatches with profile %q which does not exist (%v) — dispatch would die at launch",
			a.profilePath(), err)
	}
}

func TestWithRetryAdjudicator(t *testing.T) {
	var _ RetryAdjudicator = (*bridgeRetryAdjudicator)(nil)
	want := NewBridgeRetryAdjudicator(nil, AgentIdentity{}, "")

	o := NewOrchestrator(nil, nil, nil, WithRetryAdjudicator(want))

	if o.retryAdjudicator == nil {
		t.Fatal("WithRetryAdjudicator did not reach the field; the adjudicator would never be consulted")
	}
	if plain := NewOrchestrator(nil, nil, nil); plain.retryAdjudicator != nil {
		t.Error("an un-optioned orchestrator must have no adjudicator; judgment is opt-in")
	}
}

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
