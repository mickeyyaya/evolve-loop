package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResolveOverlays_FamiliesSelectorMatchesTheCLIFamilyOnly(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Families: []string{"claude", "agy"}, Skills: []string{"notes"}},
		{Skills: []string{"all"}},
	}}}
	cases := []struct {
		cli  string
		want []string
	}{
		{"claude", []string{"notes", "all"}},
		{"claude-tmux", []string{"notes", "all"}},
		{"agy-p", []string{"notes", "all"}},
		{"codex-tmux", []string{"all"}},
		{"my-claude", []string{"all"}},
		{"Claude", []string{"all"}},
		{"", []string{"all"}},
		{"-tmux", []string{"all"}},
	}
	for _, tc := range cases {
		got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", CLI: tc.cli})
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("cli=%q resolved %v, want %v", tc.cli, got, tc.want)
		}
	}
}
