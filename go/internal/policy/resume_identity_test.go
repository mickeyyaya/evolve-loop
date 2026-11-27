package policy_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResumeIdentityStage_DefaultsToShadowAndReadsEachWord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want policy.ResumeIdentityStage
	}{
		{"no policy", `{}`, policy.ResumeIdentityShadow},
		{"empty resume", `{"resume":{}}`, policy.ResumeIdentityShadow},
		{"empty identity", `{"resume":{"identity":{}}}`, policy.ResumeIdentityShadow},
		{"off", `{"resume":{"identity":{"stage":"off"}}}`, policy.ResumeIdentityOff},
		{"shadow", `{"resume":{"identity":{"stage":"shadow"}}}`, policy.ResumeIdentityShadow},
		{"enforce", `{"resume":{"identity":{"stage":"enforce"}}}`, policy.ResumeIdentityEnforce},
	}
	for _, tc := range tests {
		p, err := policy.Parse([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := p.ResumeIdentityStage(); got != tc.want {
			t.Errorf("%s: stage %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResumeIdentityPolicy_ParsesIntoTheResumeBlockBesideTheCheckpointAge(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"resume":{"max_checkpoint_age_h":24,"identity":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := &policy.ResumeConfig{MaxCheckpointAgeH: 24, Identity: &policy.ResumeIdentityPolicy{Stage: policy.ResumeIdentityEnforce}}
	if !reflect.DeepEqual(p.Resume, want) {
		t.Errorf("resume = %+v, want %+v", p.Resume, want)
	}
}

func TestResumeIdentityStage_RefusesAnyOtherWordAndAnUnknownKey(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"resume":{"identity":{"stage":"on"}}}`,
		`{"resume":{"identity":{"stage":"Enforce"}}}`,
		`{"resume":{"other":{}}}`,
		`{"resume":{"identity":{"stag":"shadow"}}}`,
	} {
		if _, err := policy.Parse([]byte(raw)); err == nil {
			t.Errorf("%s: want a refusal", raw)
		}
	}
}

func TestResumeIdentityStage_IsANamedStageKeyAndRoundTrips(t *testing.T) {
	t.Parallel()
	found := false
	for _, f := range policy.StageFields() {
		if f.Path == "resume.identity.stage" {
			found = true
			if f.TypeName != reflect.TypeOf(policy.ResumeIdentityStage("")).String() {
				t.Errorf("type name %q, want the named ResumeIdentityStage", f.TypeName)
			}
		}
	}
	if !found {
		t.Fatal("StageFields lacks resume.identity.stage")
	}
	p, err := policy.Parse([]byte(`{"resume":{"identity":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(p.Resume)
	if err != nil || string(out) != `{"identity":{"stage":"enforce"}}` {
		t.Fatalf("resume JSON = %s, %v", out, err)
	}
}
