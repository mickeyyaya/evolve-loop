package retryladder_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestAmplify_NextIsDeterministicAndDoesNotMutateItsInput(t *testing.T) {
	for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
		in := capabilityAt(rung, "codes:a", "codes:a", "codes:b")
		in.Chain = []string{"agy", "agy-claude", "claude"}
		in.Headroom = fullHeadroom()
		earlier := slices.Clone(in.Earlier)
		chain := slices.Clone(in.Chain)
		first := retryladder.Next(in)
		second := retryladder.Next(in)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("rung %s: Next is not deterministic: %+v vs %+v", rung, first, second)
		}
		if !slices.Equal(in.Earlier, earlier) || !slices.Equal(in.Chain, chain) {
			t.Errorf("rung %s: Next mutated its input: earlier %v chain %v", rung, in.Earlier, in.Chain)
		}
		if !reflect.DeepEqual(in.Headroom, fullHeadroom()) {
			t.Errorf("rung %s: Next mutated the headroom table", rung)
		}
	}
}

func TestAmplify_EmptyAndNilTablesNeverPanicOrExceedTwoRaises(t *testing.T) {
	tables := map[string]convergence.HeadroomTable{"nil": nil, "empty": {}}
	chains := map[string][]string{"nil": nil, "empty": {}, "unknown family": {"nonexistent"}}
	for tn, headroom := range tables {
		for cn, chain := range chains {
			for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
				in := capabilityAt(rung, "codes:a", "codes:a")
				in.Headroom, in.Chain = headroom, chain
				got := retryladder.Next(in)
				if got.Action == retryladder.ActionRaise && got.Rung == retryladder.R2 && rung != retryladder.R1 && rung != retryladder.R0 {
					t.Errorf("headroom %s chain %s at %s: raise to R2 from %s", tn, cn, rung, rung)
				}
				if got.Action == retryladder.ActionRaise && got.Rung == retryladder.R2 && got.Target.Family != "" && len(headroom) == 0 {
					t.Errorf("headroom %s chain %s at %s: raised to a family %q with no headroom", tn, cn, rung, got.Target.Family)
				}
			}
		}
	}
}

func TestAmplify_R1CapabilityNeverRaisesToR2(t *testing.T) {
	for _, earlier := range [][]string{nil, {"codes:a"}, {"codes:z"}} {
		got := retryladder.Next(capabilityAt(retryladder.R1, "codes:a", earlier...))
		if got.Action == retryladder.ActionRaise && got.Rung != retryladder.R2 {
			t.Errorf("earlier %v: raise landed on %s, want R2 or a stop", earlier, got.Rung)
		}
		if got.Action == retryladder.ActionRaise && got.Rung == retryladder.R2 && got.Class != retryladder.Capability {
			t.Errorf("earlier %v: raise class = %s, want capability", earlier, got.Class)
		}
	}
}

func TestAmplify_StopAtR2OnEarlierFingerprintCarriesTheSpecSignal(t *testing.T) {
	got := retryladder.Next(capabilityAt(retryladder.R2, "codes:a", "codes:b", "codes:a"))
	if got.Action != retryladder.ActionStop || got.Class != retryladder.Spec {
		t.Fatalf("R2 on an earlier fingerprint = {%s %s}, want a stop as spec", got.Action, got.Class)
	}
	if !slices.Contains(got.Signals, retryladder.SignalStopSpec) {
		t.Errorf("signals = %v, want %s", got.Signals, retryladder.SignalStopSpec)
	}
	if slices.Contains(got.Signals, retryladder.SignalNoHeadroom) {
		t.Errorf("signals = %v, a spec stop must not claim missing headroom", got.Signals)
	}
}

func TestAmplify_InfraAndQuotaNeverEmitLadderSignals(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Infra, retryladder.Quota} {
		in := capabilityAt(retryladder.R1, "codes:a", "codes:a")
		in.Verdict.Class = class
		in.Headroom = noDeepHeadroom()
		got := retryladder.Next(in)
		for _, sig := range []string{retryladder.SignalNoHeadroom, retryladder.SignalStopSpec} {
			if slices.Contains(got.Signals, sig) {
				t.Errorf("%s: signals %v contain %s on a same-rung retry", class, got.Signals, sig)
			}
		}
	}
}

func TestAmplify_BoundsResolvedIsIdempotent(t *testing.T) {
	cases := []retryladder.Bounds{{}, {MaxTier: "top"}, {MaxEffort: "xhigh"}, {MaxTier: "balanced", MaxEffort: "low"}}
	for _, b := range cases {
		once := b.Resolved()
		if twice := once.Resolved(); twice != once {
			t.Errorf("Resolved not idempotent for %+v: %+v then %+v", b, once, twice)
		}
	}
}
