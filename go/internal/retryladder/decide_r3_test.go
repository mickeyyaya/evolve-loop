package retryladder_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type recordedRun struct {
	cycle    int
	failures []string
}

var recordedRuns = []recordedRun{
	{cycle: 1972, failures: []string{
		"audit FAIL round 1: H1 the mixed todo loses its full reach, M1 the named design docs are not updated",
		"audit FAIL round 2: H1 carried over unrepaired, M1 carried over unrepaired",
		"audit FAIL round 3: H1 and M1 repaired, the explanation document is inaccurate",
	}},
	{cycle: 1983, failures: []string{
		"audit FAIL round 1: three tests in core/failurelearning fail after the shared minter change",
		"audit FAIL round 2: H1 failurelearning floor tests still red, H2 build-report regression slice fixed",
		"audit FAIL round 3: the same H1 unchanged, this round changed no production code",
	}},
}

func replayLadder(failures []string) []retryladder.Decision {
	rung, earlier := retryladder.R0, []string(nil)
	var decisions []retryladder.Decision
	for _, reason := range failures {
		fingerprint := retryladder.Fingerprint(reason)
		decision := retryladder.Next(capabilityAt(rung, fingerprint, earlier...))
		decisions = append(decisions, decision)
		if decision.Action != retryladder.ActionRaise {
			break
		}
		rung, earlier = decision.Rung, append(earlier, fingerprint)
	}
	return decisions
}

func TestNext_ReplayCycles1972And1983(t *testing.T) {
	for _, run := range recordedRuns {
		decisions := replayLadder(run.failures)
		var rungs []retryladder.Rung
		for _, decision := range decisions {
			if decision.Action != retryladder.ActionRaise {
				t.Fatalf("cycle %d: decision %+v stopped before R3", run.cycle, decision)
			}
			rungs = append(rungs, decision.Rung)
		}
		want := []retryladder.Rung{retryladder.R1, retryladder.R2, retryladder.R3}
		if !slices.Equal(rungs, want) {
			t.Errorf("cycle %d: rungs = %v, want %v", run.cycle, rungs, want)
		}
	}
}

func TestNext_R3SkipsR2WhenTheR1FingerprintRepeats(t *testing.T) {
	tests := []struct {
		name     string
		in       retryladder.Input
		wantRung retryladder.Rung
	}{
		{"R1 repeats the fingerprint of R0", capabilityAt(retryladder.R1, "codes:a", "codes:a"), retryladder.R3},
		{"R1 repeats a fingerprint from further back", capabilityAt(retryladder.R1, "codes:a", "codes:z", "codes:a"), retryladder.R3},
		{"R1 with a new fingerprint still climbs to R2", capabilityAt(retryladder.R1, "codes:b", "codes:a"), retryladder.R2},
		{"R1 with an empty fingerprint still climbs to R2", capabilityAt(retryladder.R1, "", "codes:a"), retryladder.R2},
		{"R0 on a repeated fingerprint still goes to R2", capabilityAt(retryladder.R0, "codes:a", "codes:a"), retryladder.R2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryladder.Next(tt.in)
			if got.Action != retryladder.ActionRaise || got.Rung != tt.wantRung || got.Class != retryladder.Capability {
				t.Fatalf("Next = {%s %s %s}, want a capability raise to %s", got.Action, got.Rung, got.Class, tt.wantRung)
			}
			if slices.Contains(got.Signals, retryladder.SignalNoHeadroom) {
				t.Errorf("Signals = %v: a skip by fingerprint must not claim missing headroom", got.Signals)
			}
		})
	}
}

func TestNext_R3SkipsR2WhenNoEffortStepAndNoTierHeadroom(t *testing.T) {
	agyNoDeep := capabilityAt(retryladder.R1, "codes:b", "codes:a")
	agyNoDeep.Base, agyNoDeep.Chain, agyNoDeep.Headroom = agySeat(), buildChain, noDeepHeadroom()

	claudeAtEffortCap := capabilityAt(retryladder.R1, "codes:b", "codes:a")
	claudeAtEffortCap.Base, claudeAtEffortCap.Headroom = claudeSeat("balanced", "high"), noDeepHeadroom()

	deepAtTierCap := capabilityAt(retryladder.R1, "codes:b", "codes:a")
	deepAtTierCap.Base = claudeSeat("deep", "high")

	agyWithDeep := capabilityAt(retryladder.R1, "codes:b", "codes:a")
	agyWithDeep.Base, agyWithDeep.Chain = agySeat(), buildChain

	tests := []struct {
		name        string
		in          retryladder.Input
		wantRung    retryladder.Rung
		wantSignals []string
	}{
		{"agy has no effort step and no deep family has headroom", agyNoDeep, retryladder.R3, []string{retryladder.SignalNoHeadroom}},
		{"claude is at max_effort and no deep family has headroom", claudeAtEffortCap, retryladder.R3, []string{retryladder.SignalNoHeadroom}},
		{"the base tier is already max_tier", deepAtTierCap, retryladder.R3, nil},
		{"agy has no effort step but a deep family has headroom, so R2 takes the tier step", agyWithDeep, retryladder.R2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryladder.Next(tt.in)
			if got.Action != retryladder.ActionRaise || got.Rung != tt.wantRung || got.Class != retryladder.Capability {
				t.Fatalf("Next = {%s %s %s}, want a capability raise to %s", got.Action, got.Rung, got.Class, tt.wantRung)
			}
			if !slices.Equal(got.Signals, tt.wantSignals) {
				t.Errorf("Signals = %v, want %v", got.Signals, tt.wantSignals)
			}
		})
	}
}

func TestNext_R3FollowsAFailedR2WhateverTheFingerprint(t *testing.T) {
	tests := []struct {
		name string
		in   retryladder.Input
	}{
		{"a new fingerprint", capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b")},
		{"a repeated fingerprint", capabilityAt(retryladder.R2, "codes:a", "codes:a", "codes:b")},
		{"no headroom at all", func() retryladder.Input {
			in := capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b")
			in.Headroom = nil
			return in
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryladder.Next(tt.in)
			if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R3 || got.Class != retryladder.Capability {
				t.Fatalf("Next = {%s %s %s}, want a capability raise to R3", got.Action, got.Rung, got.Class)
			}
			if slices.Contains(got.Signals, retryladder.SignalStopSpec) {
				t.Errorf("Signals = %v: R2 must hand over to R3, not stop as spec", got.Signals)
			}
		})
	}
}

func TestNext_R3BuilderFamilyChoice(t *testing.T) {
	balanced := func(model string) map[string]convergence.ModelEffort {
		return map[string]convergence.ModelEffort{"balanced": {Model: model}}
	}
	claudeOnlyBalanced := map[string]convergence.ModelEffort{"balanced": {Model: "sonnet", Effort: "high"}}

	tests := []struct {
		name       string
		base       convergence.TierEffort
		chain      []string
		headroom   convergence.HeadroomTable
		rung       retryladder.Rung
		fingerprnt string
		earlier    []string
		wantFamily string
		wantModel  string
	}{
		{
			name: "another family with headroom at the base tier takes R3", base: claudeSeat("balanced", "medium"),
			chain:    []string{"claude", "agy"},
			headroom: convergence.HeadroomTable{"claude": claudeOnlyBalanced, "agy": balanced("flash-high")},
			rung:     retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "agy", wantModel: "flash-high",
		},
		{
			name: "the same choice holds when R3 is reached by skipping R2", base: claudeSeat("balanced", "medium"),
			chain:    []string{"claude", "agy"},
			headroom: convergence.HeadroomTable{"claude": claudeOnlyBalanced, "agy": balanced("flash-high")},
			rung:     retryladder.R1, fingerprnt: "codes:a", earlier: []string{"codes:a"},
			wantFamily: "agy", wantModel: "flash-high",
		},
		{
			name: "the order of the chain decides between two healthy families", base: claudeSeat("balanced", "medium"),
			chain:    []string{"claude", "agy-claude", "agy"},
			headroom: convergence.HeadroomTable{"claude": claudeOnlyBalanced, "agy-claude": balanced("agy-sonnet"), "agy": balanced("flash-high")},
			rung:     retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "agy-claude", wantModel: "agy-sonnet",
		},
		{
			name: "a family without headroom at the base tier is skipped", base: claudeSeat("balanced", "medium"),
			chain: []string{"claude", "agy", "agy-claude"},
			headroom: convergence.HeadroomTable{
				"claude": claudeOnlyBalanced, "agy": {"deep": {Model: "argon-medium"}}, "agy-claude": balanced("agy-sonnet"),
			},
			rung: retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "agy-claude", wantModel: "agy-sonnet",
		},
		{
			name: "no other family has headroom, so R3 stays on the base family", base: claudeSeat("balanced", "medium"),
			chain:    []string{"claude", "agy"},
			headroom: convergence.HeadroomTable{"claude": claudeOnlyBalanced, "agy": {"deep": {Model: "argon-medium"}}},
			rung:     retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "claude", wantModel: "sonnet",
		},
		{
			name: "the base family is never the other family", base: claudeSeat("balanced", "medium"),
			chain:    []string{"claude", "agy"},
			headroom: convergence.HeadroomTable{"claude": claudeOnlyBalanced},
			rung:     retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "claude", wantModel: "sonnet",
		},
		{
			name: "an agy base moves to the first other family that serves the base tier", base: agySeat(),
			chain: buildChain,
			headroom: convergence.HeadroomTable{
				"agy": {"balanced": {Model: "flash-high"}}, "agy-claude": {"top": {Model: "agy-opus-top"}, "deep": {Model: "agy-opus"}},
				"claude": claudeOnlyBalanced,
			},
			rung: retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "claude", wantModel: "sonnet",
		},
		{
			name: "an empty headroom table keeps the base family", base: claudeSeat("balanced", "medium"),
			chain: []string{"claude", "agy"}, headroom: convergence.HeadroomTable{},
			rung: retryladder.R2, fingerprnt: "codes:c", earlier: []string{"codes:a", "codes:b"},
			wantFamily: "claude", wantModel: "balanced-model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capabilityAt(tt.rung, tt.fingerprnt, tt.earlier...)
			in.Base, in.Chain, in.Headroom = tt.base, tt.chain, tt.headroom
			got := retryladder.Next(in)
			if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R3 {
				t.Fatalf("Next = {%s %s}, want a raise to R3", got.Action, got.Rung)
			}
			if got.Target.Family != tt.wantFamily || got.Target.Model != tt.wantModel || got.Target.Tier != tt.base.Tier {
				t.Errorf("Target = %+v, want family %s model %s tier %s", got.Target, tt.wantFamily, tt.wantModel, tt.base.Tier)
			}
			if got.Target.Effort != "" && effortIndex(got.Target.Effort) > effortIndex(retryladder.DefaultMaxEffort) {
				t.Errorf("Target effort %q is above the default max_effort", got.Target.Effort)
			}
		})
	}
}

func TestNext_R3IsTheLastRung(t *testing.T) {
	for name, headroom := range map[string]convergence.HeadroomTable{"full": fullHeadroom(), "none": {}, "no deep": noDeepHeadroom()} {
		t.Run("a new fingerprint stops as capability with "+name+" headroom", func(t *testing.T) {
			in := capabilityAt(retryladder.R3, "codes:d", "codes:a", "codes:b", "codes:c")
			in.Chain, in.Headroom = buildChain, headroom
			got := retryladder.Next(in)
			if got.Action != retryladder.ActionStop || got.Rung != retryladder.R3 || got.Class != retryladder.Capability {
				t.Fatalf("Next = {%s %s %s}, want a capability stop at R3", got.Action, got.Rung, got.Class)
			}
			if len(got.Signals) != 0 || got.Target != (convergence.TierEffort{}) {
				t.Errorf("a stop carries no signals and no target: %+v", got)
			}
		})
	}

	repeated := retryladder.Next(capabilityAt(retryladder.R3, "codes:a", "codes:a", "codes:b", "codes:c"))
	if repeated.Action != retryladder.ActionStop || repeated.Rung != retryladder.R3 || repeated.Class != retryladder.Spec ||
		!slices.Equal(repeated.Signals, []string{retryladder.SignalStopSpec}) {
		t.Errorf("R3 on a repeated fingerprint = %+v, want a stop as spec with %s", repeated, retryladder.SignalStopSpec)
	}
}

func TestNext_R3KeepsTheClassRulesOfEveryOtherRung(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Halt, retryladder.Spec, retryladder.Unknown} {
		in := capabilityAt(retryladder.R3, "codes:a", "codes:b")
		in.Verdict.Class = class
		got := retryladder.Next(in)
		if got.Action != retryladder.ActionStop || got.Rung != retryladder.R3 || got.Class != class {
			t.Errorf("%s at R3: Next = {%s %s %s}, want a stop at R3 as %s", class, got.Action, got.Rung, got.Class, class)
		}
	}
	for _, class := range []retryladder.Class{retryladder.Infra, retryladder.Quota} {
		in := capabilityAt(retryladder.R3, "codes:a", "codes:b")
		in.Verdict.Class = class
		got := retryladder.Next(in)
		if got.Action != retryladder.ActionRetry || got.Rung != retryladder.R3 || got.Class != class {
			t.Errorf("%s at R3: Next = {%s %s %s}, want a retry on R3", class, got.Action, got.Rung, got.Class)
		}
	}
}
