package cliroute_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestRouter_EveryDecisionOwnsItsTriggers(t *testing.T) {
	r := mustBuild(t, cliroute.Setup{Profiles: syntheticProfiles(t), Host: cliroute.Host{LookPath: everyBinary()}})
	want := llmroute.DefaultTriggers()
	for label, req := range map[string]cliroute.Request{
		"dispatch launch": {Agent: "scout", Phase: "scout", DefaultModel: "balanced"},
		"advisor launch":  {Agent: "scout", Phase: "scout", Launch: cliroute.LaunchAdvisor},
	} {
		decide := func() cliroute.Decision {
			d, err := r.Resolve(req)
			if err != nil || !d.Legacy() || len(d.Plan.Triggers) < 3 {
				t.Fatalf("%s: Resolve = %+v (err %v), want a legacy decision with the default triggers", label, d, err)
			}
			return d
		}
		edited := decide()
		edited.Plan.Triggers[0] = 999
		_ = append(edited.Plan.Triggers[:2], 4242)

		if got := decide().Plan.Triggers; !slices.Equal(got, want) {
			t.Errorf("%s: a later decision's Triggers = %v after the caller edited an earlier one's, want %v", label, got, want)
		}
		if got := llmroute.DefaultTriggers(); !slices.Equal(got, want) {
			t.Errorf("%s: the package default became %v", label, got)
		}
	}
}
