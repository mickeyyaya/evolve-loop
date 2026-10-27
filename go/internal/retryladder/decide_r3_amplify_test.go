package retryladder_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestAmplifyR3_NextDoesNotMutateItsInput(t *testing.T) {
	in := capabilityAt(retryladder.R1, "codes:a", "codes:a")
	in.Chain = []string{"claude", "agy"}
	chainBefore := append([]string(nil), in.Chain...)
	earlierBefore := append([]string(nil), in.Earlier...)
	retryladder.Next(in)
	if !reflect.DeepEqual(in.Chain, chainBefore) || !reflect.DeepEqual(in.Earlier, earlierBefore) {
		t.Errorf("Next mutated its input: chain %v earlier %v", in.Chain, in.Earlier)
	}
}

func TestAmplifyR3_NextIsDeterministic(t *testing.T) {
	in := capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b")
	in.Chain = buildChain
	first := retryladder.Next(in)
	for i := 0; i < 20; i++ {
		if got := retryladder.Next(in); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d: %+v differs from %+v", i, got, first)
		}
	}
}

func TestAmplifyR3_ReplayTerminatesWithinThreeRaises(t *testing.T) {
	fingerprints := []string{"", "codes:a", "codes:b"}
	for _, a := range fingerprints {
		for _, b := range fingerprints {
			for _, c := range fingerprints {
				for _, d := range fingerprints {
					decisions := replayLadder([]string{a, b, c, d, a, b})
					raises := 0
					for _, decision := range decisions {
						if decision.Action == retryladder.ActionRaise {
							raises++
						}
					}
					if raises > 3 || decisions[len(decisions)-1].Action == retryladder.ActionRaise {
						t.Fatalf("%q %q %q %q: %d raises, last %+v", a, b, c, d, raises, decisions[len(decisions)-1])
					}
				}
			}
		}
	}
}

func TestAmplifyR3_TargetNeverBelowBaseTierOrAboveMaxEffort(t *testing.T) {
	for _, base := range []string{"balanced", "deep", "top"} {
		in := capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b")
		in.Base = claudeSeat(base, "high")
		got := retryladder.Next(in)
		if got.Action != retryladder.ActionRaise {
			continue
		}
		if got.Rung == retryladder.R3 && got.Target.Tier != base {
			t.Errorf("base %s: R3 tier = %s", base, got.Target.Tier)
		}
		if got.Target.Effort != "" && effortIndex(got.Target.Effort) > effortIndex(retryladder.DefaultMaxEffort) {
			t.Errorf("base %s: effort %q above max_effort", base, got.Target.Effort)
		}
	}
}

func TestAmplifyR3_EmptyChainAndNilHeadroomStillReachR3(t *testing.T) {
	in := capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b")
	in.Chain, in.Headroom = nil, nil
	got := retryladder.Next(in)
	if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R3 || got.Target.Family != "claude" {
		t.Errorf("Next = %+v, want a raise to R3 on the base family", got)
	}
}
