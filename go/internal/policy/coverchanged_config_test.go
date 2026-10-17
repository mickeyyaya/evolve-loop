package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCoverChangedConfig_EnforcesUntilThePolicySaysOtherwise(t *testing.T) {
	for name, tc := range map[string]struct {
		p    policy.Policy
		want string
	}{
		"an absent block enforces": {policy.Policy{}, "enforce"},
		"an empty block enforces":  {policy.Policy{CoverChanged: &policy.CoverChangedPolicy{}}, "enforce"},
		"an explicit shadow wins":  {policy.Policy{CoverChanged: &policy.CoverChangedPolicy{Stage: "shadow"}}, "shadow"},
		"an explicit off wins":     {policy.Policy{CoverChanged: &policy.CoverChangedPolicy{Stage: "off"}}, "off"},
	} {
		if got := tc.p.CoverChangedConfig().Stage; got != tc.want {
			t.Errorf("%s: stage = %q, want %q", name, got, tc.want)
		}
	}
}

func TestCoverChangedConfig_TheStageReachesTheFloorFromPolicyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"cover_changed":{"stage":"shadow"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.CoverChangedConfig().Stage; got != "shadow" {
		t.Errorf("stage = %q, want shadow", got)
	}
}
