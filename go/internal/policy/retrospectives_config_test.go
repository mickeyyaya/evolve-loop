package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRetrospectivesConfig_ShadowsUntilThePolicySaysOtherwise(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		p    policy.Policy
		want string
	}{
		"no block shadows":         {policy.Policy{}, "shadow"},
		"an empty block shadows":   {policy.Policy{Retrospectives: &policy.RetrospectivesPolicy{}}, "shadow"},
		"an explicit enforce wins": {policy.Policy{Retrospectives: &policy.RetrospectivesPolicy{Stage: "enforce"}}, "enforce"},
		"an explicit off wins":     {policy.Policy{Retrospectives: &policy.RetrospectivesPolicy{Stage: "off"}}, "off"},
	}
	for name, tc := range cases {
		if got := tc.p.RetrospectivesConfig().Stage; got != tc.want {
			t.Errorf("%s: stage = %q, want %q", name, got, tc.want)
		}
	}
}

func TestRetrospectivesConfig_TheStrictParserReadsTheStageAndRefusesAnUnknownSubkey(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"retrospectives":{"stage":"enforce"}}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := p.RetrospectivesConfig().Stage; got != "enforce" {
		t.Fatalf("stage = %q, want enforce", got)
	}
	if _, err := policy.Parse([]byte(`{"retrospectives":{"stgae":"enforce"}}`)); err == nil {
		t.Fatal("Parse accepted retrospectives.stgae, want the unknown-key refusal")
	}
}
