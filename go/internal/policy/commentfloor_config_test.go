package policy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCommentFloorConfig_ShadowsUntilThePolicySaysOtherwise(t *testing.T) {
	for name, tc := range map[string]struct {
		p    policy.Policy
		want string
	}{
		"an absent block shadows":  {policy.Policy{}, "shadow"},
		"an empty block shadows":   {policy.Policy{CommentFloor: &policy.CommentFloorPolicy{}}, "shadow"},
		"an explicit enforce wins": {policy.Policy{CommentFloor: &policy.CommentFloorPolicy{Stage: "enforce"}}, "enforce"},
		"an explicit off wins":     {policy.Policy{CommentFloor: &policy.CommentFloorPolicy{Stage: "off"}}, "off"},
	} {
		if got := tc.p.CommentFloorConfig().Stage; got != tc.want {
			t.Errorf("%s: stage = %q, want %q", name, got, tc.want)
		}
	}
}

func TestCommentFloorConfig_TheStageReachesTheFloorFromPolicyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"comment_floor":{"stage":"enforce"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.CommentFloorConfig().Stage; got != "enforce" {
		t.Errorf("stage = %q, want enforce", got)
	}
}
