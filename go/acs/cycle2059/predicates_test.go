//go:build acs

package cycle2059

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func claudeBalanced(effort string) convergence.TierEffort {
	return convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "balanced-model", Effort: effort}
}

func headroomOf(families map[string]string) convergence.HeadroomTable {
	table := convergence.HeadroomTable{}
	for family, tier := range families {
		table[family] = map[string]convergence.ModelEffort{tier: {Model: family + "-" + tier, Effort: "high"}}
	}
	return table
}

func capabilityAfter(rung retryladder.Rung, reason string, earlier ...string) retryladder.Input {
	return retryladder.Input{
		Rung:     rung,
		Earlier:  earlier,
		Verdict:  retryladder.Verdict{Class: retryladder.Capability, Fingerprint: retryladder.Fingerprint(reason)},
		Base:     claudeBalanced("medium"),
		Chain:    []string{"claude"},
		Headroom: convergence.HeadroomTable{"claude": {"balanced": {Model: "sonnet", Effort: "high"}, "deep": {Model: "opus", Effort: "high"}}},
	}
}

func replay(failures []string) []retryladder.Rung {
	rung, earlier := retryladder.R0, []string(nil)
	var rungs []retryladder.Rung
	for _, reason := range failures {
		decision := retryladder.Next(capabilityAfter(rung, reason, earlier...))
		if decision.Action != retryladder.ActionRaise {
			rungs = append(rungs, "stop:"+decision.Rung)
			break
		}
		rungs = append(rungs, decision.Rung)
		rung, earlier = decision.Rung, append(earlier, retryladder.Fingerprint(reason))
	}
	return rungs
}

func TestC2059_001_ReplayOfCycles1972And1983ClimbsR1ThenR2ThenR3(t *testing.T) {
	runs := map[string][]string{
		"cycle 1972": {
			"audit FAIL round 1: H1 the mixed todo loses its full reach, M1 the named design docs are not updated",
			"audit FAIL round 2: H1 carried over unrepaired, M1 carried over unrepaired",
			"audit FAIL round 3: H1 and M1 repaired, the explanation document is inaccurate",
		},
		"cycle 1983": {
			"audit FAIL round 1: three tests in core/failurelearning fail after the shared minter change",
			"audit FAIL round 2: H1 failurelearning floor tests still red, H2 build-report regression slice fixed",
			"audit FAIL round 3: the same H1 unchanged, this round changed no production code",
		},
	}
	want := []retryladder.Rung{retryladder.R1, retryladder.R2, retryladder.R3}
	for name, failures := range runs {
		if got := replay(failures); !slices.Equal(got, want) {
			t.Errorf("%s: rungs = %v, want %v", name, got, want)
		}
	}
}

func TestC2059_002_RepeatedR1FingerprintSkipsR2(t *testing.T) {
	repeated := retryladder.Next(capabilityAfter(retryladder.R1, "[floor_red] 3 tests fail", retryladder.Fingerprint("[floor_red] 3 tests fail")))
	if repeated.Action != retryladder.ActionRaise || repeated.Rung != retryladder.R3 {
		t.Errorf("R1 repeating its fingerprint = {%s %s}, want a raise to R3", repeated.Action, repeated.Rung)
	}
	fresh := retryladder.Next(capabilityAfter(retryladder.R1, "[other_red] 1 test fails", retryladder.Fingerprint("[floor_red] 3 tests fail")))
	if fresh.Action != retryladder.ActionRaise || fresh.Rung != retryladder.R2 {
		t.Errorf("R1 with a new fingerprint = {%s %s}, want a raise to R2", fresh.Action, fresh.Rung)
	}
}

func TestC2059_003_NoEffortStepAndNoTierHeadroomSkipsR2(t *testing.T) {
	stuck := capabilityAfter(retryladder.R1, "[a] red", retryladder.Fingerprint("[b] red"))
	stuck.Base = claudeBalanced("high")
	stuck.Headroom = headroomOf(map[string]string{"claude": "balanced"})
	got := retryladder.Next(stuck)
	if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R3 {
		t.Errorf("no effort step and no tier headroom = {%s %s}, want a raise to R3", got.Action, got.Rung)
	}
	withDeep := stuck
	withDeep.Headroom = headroomOf(map[string]string{"claude": "deep"})
	if climbed := retryladder.Next(withDeep); climbed.Rung != retryladder.R2 {
		t.Errorf("tier headroom present: rung = %s, want R2", climbed.Rung)
	}
}

func TestC2059_004_R3BuilderRunsOnAnotherFamilyOnlyWhenItHasHeadroom(t *testing.T) {
	healthy := capabilityAfter(retryladder.R2, "[a] red", retryladder.Fingerprint("[b] red"))
	healthy.Chain = []string{"claude", "agy"}
	healthy.Headroom = headroomOf(map[string]string{"claude": "balanced", "agy": "balanced"})
	got := retryladder.Next(healthy)
	if got.Rung != retryladder.R3 || got.Target.Family != "agy" || got.Target.Tier != "balanced" {
		t.Errorf("another family with headroom: Next = %+v, want R3 on agy at balanced", got)
	}
	benched := healthy
	benched.Headroom = headroomOf(map[string]string{"claude": "balanced", "agy": "deep"})
	stay := retryladder.Next(benched)
	if stay.Rung != retryladder.R3 || stay.Target.Family != "claude" {
		t.Errorf("the other family has no headroom: Next = %+v, want R3 on claude", stay)
	}
}

func TestC2059_005_LadderStopsAfterR3(t *testing.T) {
	for name, headroom := range map[string]convergence.HeadroomTable{
		"full":  headroomOf(map[string]string{"claude": "balanced", "agy": "balanced"}),
		"empty": {},
	} {
		in := capabilityAfter(retryladder.R3, "[d] red", retryladder.Fingerprint("[a] red"))
		in.Chain, in.Headroom = []string{"claude", "agy"}, headroom
		got := retryladder.Next(in)
		if got.Action != retryladder.ActionStop || got.Rung != retryladder.R3 {
			t.Errorf("%s headroom: Next = {%s %s}, want a stop at R3", name, got.Action, got.Rung)
		}
	}
}

func TestC2059_006_ContractCorrectionLoopDispatchesR3ThroughTheProductionCaller(t *testing.T) {
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, stderr, code, runErr := acsassert.SubprocessOutput("go", "test", "-C", moduleDir, "-count=1", "-v",
		"-run", "^TestContractCorrection_EnforceEscalatesToAFreshRaisedTryAfterOneCorrection$", "./internal/core")
	if runErr != nil || code != 0 {
		t.Fatalf("core enforce-ladder test exit=%d: %v\n%s", code, runErr, acsassert.OutputTail(out+stderr, 12))
	}
	if !strings.Contains(out, "--- PASS: TestContractCorrection_EnforceEscalatesToAFreshRaisedTryAfterOneCorrection") {
		t.Errorf("the core test did not run:\n%s", acsassert.OutputTail(out, 8))
	}
}
