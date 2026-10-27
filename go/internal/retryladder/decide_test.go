package retryladder_test

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

var buildChain = []string{"agy", "agy-claude", "claude"}

func claudeSeat(tier, effort string) convergence.TierEffort {
	return convergence.TierEffort{Family: "claude", Tier: tier, Model: tier + "-model", Effort: effort}
}

func agySeat() convergence.TierEffort {
	return convergence.TierEffort{Family: "agy", Tier: "balanced", Model: "flash-high"}
}

func fullHeadroom() convergence.HeadroomTable {
	return convergence.HeadroomTable{
		"agy":        {"balanced": {Model: "flash-high"}, "deep": {Model: "argon-medium"}},
		"agy-claude": {"top": {Model: "agy-opus-top"}, "deep": {Model: "agy-opus"}},
		"claude": {
			"balanced": {Model: "sonnet", Effort: "high"},
			"deep":     {Model: "opus", Effort: "high"},
			"top":      {Model: "opus-top", Effort: "xhigh"},
		},
	}
}

func noDeepHeadroom() convergence.HeadroomTable {
	return convergence.HeadroomTable{
		"agy":    {"balanced": {Model: "flash-high"}},
		"claude": {"balanced": {Model: "sonnet", Effort: "high"}},
	}
}

func capabilityAt(rung retryladder.Rung, fingerprint string, earlier ...string) retryladder.Input {
	return retryladder.Input{
		Rung:     rung,
		Earlier:  earlier,
		Verdict:  retryladder.Verdict{Class: retryladder.Capability, Fingerprint: fingerprint},
		Base:     claudeSeat("balanced", "medium"),
		Chain:    []string{"claude"},
		Headroom: fullHeadroom(),
	}
}

func TestNext_CapabilityLadderTable(t *testing.T) {
	agyBase := capabilityAt(retryladder.R0, "codes:a")
	agyBase.Base, agyBase.Chain = agySeat(), buildChain

	agyBenchedDeep := agyBase
	agyBenchedDeep.Headroom = convergence.HeadroomTable{
		"agy":        {"balanced": {Model: "flash-high"}},
		"agy-claude": {"deep": {Model: "agy-opus"}},
	}

	noHeadroomR0 := capabilityAt(retryladder.R0, "codes:a")
	noHeadroomR0.Headroom = noDeepHeadroom()
	noHeadroomR0Repeated := capabilityAt(retryladder.R0, "codes:a", "codes:a")
	noHeadroomR0Repeated.Headroom = noDeepHeadroom()
	noHeadroomR1 := capabilityAt(retryladder.R1, "codes:a")
	noHeadroomR1.Headroom = noDeepHeadroom()
	noHeadroomR0NoStep := capabilityAt(retryladder.R0, "codes:a")
	noHeadroomR0NoStep.Base, noHeadroomR0NoStep.Headroom = agySeat(), noDeepHeadroom()
	noHeadroomR0NoStep.Chain = buildChain

	tests := []struct {
		name        string
		in          retryladder.Input
		wantAction  retryladder.Action
		wantRung    retryladder.Rung
		wantClass   retryladder.Class
		wantTarget  convergence.TierEffort
		wantSignals []string
	}{
		{
			name:       "R0 to R1: a new fingerprint and an effort step left",
			in:         capabilityAt(retryladder.R0, "codes:a"),
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R1, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "balanced-model", Effort: "high"},
		},
		{
			name:       "R0 to R2: the fingerprint repeats",
			in:         capabilityAt(retryladder.R0, "codes:a", "codes:a"),
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R2, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "deep", Model: "opus", Effort: "high"},
		},
		{
			name:       "R0 to R2: agy has no effort step, the first healthy deep family is agy",
			in:         agyBase,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R2, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "agy", Tier: "deep", Model: "argon-medium"},
		},
		{
			name:       "R0 to R2: the benched deep family is skipped for the next family of the chain",
			in:         agyBenchedDeep,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R2, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "agy-claude", Tier: "deep", Model: "agy-opus"},
		},
		{
			name:       "R1 to R2: a second raised try climbs one tier",
			in:         capabilityAt(retryladder.R1, "codes:b", "codes:a"),
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R2, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "deep", Model: "opus", Effort: "high"},
		},
		{
			name:       "R2 to R3: the failure repeats an earlier fingerprint",
			in:         capabilityAt(retryladder.R2, "codes:a", "codes:a", "codes:b"),
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R3, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "sonnet", Effort: "high"},
		},
		{
			name:       "R2 to R3: the fingerprint is new",
			in:         capabilityAt(retryladder.R2, "codes:c", "codes:a", "codes:b"),
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R3, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "sonnet", Effort: "high"},
		},
		{
			name:       "R3 stops as spec when the failure repeats an earlier fingerprint",
			in:         capabilityAt(retryladder.R3, "codes:a", "codes:a", "codes:b", "codes:c"),
			wantAction: retryladder.ActionStop, wantRung: retryladder.R3, wantClass: retryladder.Spec,
			wantSignals: []string{retryladder.SignalStopSpec},
		},
		{
			name:       "R3 stops as capability when the fingerprint is new",
			in:         capabilityAt(retryladder.R3, "codes:d", "codes:a", "codes:b", "codes:c"),
			wantAction: retryladder.ActionStop, wantRung: retryladder.R3, wantClass: retryladder.Capability,
		},
		{
			name:       "R0 to R1 on a new fingerprint needs no headroom check",
			in:         noHeadroomR0,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R1, wantClass: retryladder.Capability,
			wantTarget: convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "balanced-model", Effort: "high"},
		},
		{
			name:       "no headroom at R0 on a repeated fingerprint goes to R1 and records LADDER_NO_HEADROOM",
			in:         noHeadroomR0Repeated,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R1, wantClass: retryladder.Capability,
			wantTarget:  convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "balanced-model", Effort: "high"},
			wantSignals: []string{retryladder.SignalNoHeadroom},
		},
		{
			name:       "no headroom at R0 and no effort step goes to R1 on the same effort",
			in:         noHeadroomR0NoStep,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R1, wantClass: retryladder.Capability,
			wantTarget:  convergence.TierEffort{Family: "agy", Tier: "balanced", Model: "flash-high"},
			wantSignals: []string{retryladder.SignalNoHeadroom},
		},
		{
			name:       "no headroom at R1 skips R2 and goes to R3 on the base family",
			in:         noHeadroomR1,
			wantAction: retryladder.ActionRaise, wantRung: retryladder.R3, wantClass: retryladder.Capability,
			wantTarget:  convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "sonnet", Effort: "high"},
			wantSignals: []string{retryladder.SignalNoHeadroom},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryladder.Next(tt.in)
			if got.Action != tt.wantAction || got.Rung != tt.wantRung || got.Class != tt.wantClass {
				t.Fatalf("Next = {%s %s %s}, want {%s %s %s}", got.Action, got.Rung, got.Class, tt.wantAction, tt.wantRung, tt.wantClass)
			}
			if tt.wantAction == retryladder.ActionRaise && got.Target != tt.wantTarget {
				t.Errorf("Target = %+v, want %+v", got.Target, tt.wantTarget)
			}
			if !slices.Equal(got.Signals, tt.wantSignals) {
				t.Errorf("Signals = %v, want %v", got.Signals, tt.wantSignals)
			}
		})
	}
}

func TestNext_SignalNames(t *testing.T) {
	if retryladder.SignalNoHeadroom != "LADDER_NO_HEADROOM" || retryladder.SignalStopSpec != "LADDER_STOP_SPEC" {
		t.Fatalf("signals = %q, %q", retryladder.SignalNoHeadroom, retryladder.SignalStopSpec)
	}
}

func TestNext_EffortStepRules(t *testing.T) {
	tests := []struct {
		name       string
		base       convergence.TierEffort
		bounds     retryladder.Bounds
		headroom   convergence.HeadroomTable
		wantRung   retryladder.Rung
		wantEffort string
		wantTier   string
	}{
		{"one step up from medium to the default max_effort", claudeSeat("balanced", "medium"), retryladder.Bounds{}, fullHeadroom(), retryladder.R1, "high", "balanced"},
		{"low steps to medium", claudeSeat("balanced", "low"), retryladder.Bounds{}, fullHeadroom(), retryladder.R1, "medium", "balanced"},
		{"default max_effort high stops the step at high", claudeSeat("balanced", "high"), retryladder.Bounds{}, fullHeadroom(), retryladder.R1, "", ""},
		{"a raised max_effort allows xhigh", claudeSeat("balanced", "high"), retryladder.Bounds{MaxEffort: "xhigh"}, fullHeadroom(), retryladder.R1, "xhigh", "balanced"},
		{"max_effort medium blocks the step from medium", claudeSeat("balanced", "medium"), retryladder.Bounds{MaxEffort: "medium"}, fullHeadroom(), retryladder.R1, "", ""},
		{"max_effort medium clamps the step from low", claudeSeat("balanced", "low"), retryladder.Bounds{MaxEffort: "medium"}, fullHeadroom(), retryladder.R1, "medium", "balanced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capabilityAt(retryladder.R0, "codes:a")
			in.Base, in.Bounds, in.Headroom = tt.base, tt.bounds, tt.headroom
			got := retryladder.Next(in)
			if tt.wantEffort == "" {
				if got.Rung != retryladder.R2 {
					t.Fatalf("no effort step: Rung = %s, want R2", got.Rung)
				}
				return
			}
			if got.Rung != tt.wantRung || got.Target.Effort != tt.wantEffort || got.Target.Tier != tt.wantTier {
				t.Fatalf("Next = rung %s target %+v, want rung %s effort %s tier %s", got.Rung, got.Target, tt.wantRung, tt.wantEffort, tt.wantTier)
			}
		})
	}
}

func TestNext_TierStepRules(t *testing.T) {
	tests := []struct {
		name       string
		base       convergence.TierEffort
		bounds     retryladder.Bounds
		wantRung   retryladder.Rung
		wantTier   string
		wantEffort string
	}{
		{"default max_tier deep: balanced climbs to deep", claudeSeat("balanced", "high"), retryladder.Bounds{}, retryladder.R2, "deep", "high"},
		{"default max_tier deep: deep never climbs to top", claudeSeat("deep", "high"), retryladder.Bounds{}, retryladder.R1, "", ""},
		{"a raised max_tier top lets deep climb to top, effort clamped to the default max_effort", claudeSeat("deep", "high"), retryladder.Bounds{MaxTier: "top"}, retryladder.R2, "top", "high"},
		{"a raised max_tier and max_effort carry the default effort of the new tier", claudeSeat("deep", "high"), retryladder.Bounds{MaxTier: "top", MaxEffort: "xhigh"}, retryladder.R2, "top", "xhigh"},
		{"max_tier balanced pins a balanced base", claudeSeat("balanced", "high"), retryladder.Bounds{MaxTier: "balanced"}, retryladder.R1, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capabilityAt(retryladder.R0, "codes:a", "codes:a")
			in.Base, in.Bounds = tt.base, tt.bounds
			got := retryladder.Next(in)
			if got.Rung != tt.wantRung {
				t.Fatalf("Rung = %s, want %s (decision %+v)", got.Rung, tt.wantRung, got)
			}
			if tt.wantRung == retryladder.R2 && (got.Target.Tier != tt.wantTier || got.Target.Effort != tt.wantEffort) {
				t.Errorf("Target = %+v, want tier %s effort %s", got.Target, tt.wantTier, tt.wantEffort)
			}
		})
	}
}

func TestNext_R2EffortIsTheLargerOfTheRaisedEffortAndTheTierDefault(t *testing.T) {
	tests := []struct {
		name        string
		baseEffort  string
		tierDefault string
		want        string
	}{
		{"the stepped effort beats a lower tier default", "medium", "low", "high"},
		{"the tier default beats a lower stepped effort", "low", "high", "high"},
		{"an empty tier default keeps the stepped effort", "medium", "", "high"},
		{"a tier default above max_effort is clamped", "medium", "max", "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := capabilityAt(retryladder.R1, "codes:b", "codes:a")
			in.Base = claudeSeat("balanced", tt.baseEffort)
			in.Headroom = convergence.HeadroomTable{
				"claude": {
					"balanced": {Model: "sonnet", Effort: tt.baseEffort},
					"deep":     {Model: "opus", Effort: tt.tierDefault},
				},
			}
			got := retryladder.Next(in)
			if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R2 {
				t.Fatalf("Next = %+v, want a raise to R2", got)
			}
			if got.Target.Effort != tt.want {
				t.Errorf("R2 effort = %q, want %q", got.Target.Effort, tt.want)
			}
		})
	}
}

func TestNext_HaltAndSpecNeverClimb(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Halt, retryladder.Spec} {
		for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
			for name, headroom := range map[string]convergence.HeadroomTable{"headroom": fullHeadroom(), "none": noDeepHeadroom()} {
				in := capabilityAt(rung, "codes:a", "codes:a")
				in.Verdict.Class = class
				in.Headroom = headroom
				got := retryladder.Next(in)
				if got.Action != retryladder.ActionStop || got.Rung != rung || got.Class != class {
					t.Errorf("%s at %s with %s: Next = {%s %s %s}, want a stop at %s as %s", class, rung, name, got.Action, got.Rung, got.Class, rung, class)
				}
			}
		}
	}
}

func TestNext_UnknownNeverPassesR1(t *testing.T) {
	unknownAt := func(rung retryladder.Rung, earlier ...string) retryladder.Input {
		in := capabilityAt(rung, "codes:a", earlier...)
		in.Verdict.Class = retryladder.Unknown
		return in
	}
	r0 := retryladder.Next(unknownAt(retryladder.R0))
	if r0.Action != retryladder.ActionRaise || r0.Rung != retryladder.R1 || r0.Class != retryladder.Unknown {
		t.Errorf("unknown at R0 = {%s %s %s}, want a raise to R1", r0.Action, r0.Rung, r0.Class)
	}
	r0Repeated := retryladder.Next(unknownAt(retryladder.R0, "codes:a"))
	if r0Repeated.Rung != retryladder.R1 {
		t.Errorf("unknown at R0 on a repeated fingerprint = %s, want R1: unknown never skips to R2", r0Repeated.Rung)
	}
	for _, rung := range []retryladder.Rung{retryladder.R1, retryladder.R2} {
		got := retryladder.Next(unknownAt(rung))
		if got.Action != retryladder.ActionStop || got.Rung != rung {
			t.Errorf("unknown at %s = {%s %s}, want a stop", rung, got.Action, got.Rung)
		}
	}
}

func TestNext_InfraAndQuotaKeepTheRung(t *testing.T) {
	for _, class := range []retryladder.Class{retryladder.Infra, retryladder.Quota} {
		for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
			in := capabilityAt(rung, "codes:a", "codes:a")
			in.Verdict.Class = class
			got := retryladder.Next(in)
			if got.Action != retryladder.ActionRetry || got.Rung != rung || got.Class != class {
				t.Errorf("%s at %s: Next = {%s %s %s}, want a retry on the same rung", class, rung, got.Action, got.Rung, got.Class)
			}
		}
	}
}

func TestNext_RecordsTheShadowCriticRungWithoutFollowingIt(t *testing.T) {
	classes := []retryladder.Class{retryladder.Infra, retryladder.Quota, retryladder.Halt, retryladder.Spec, retryladder.Capability, retryladder.Unknown}
	for _, class := range classes {
		for _, rung := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
			in := capabilityAt(rung, "codes:a")
			in.Verdict.Class = class
			without := retryladder.Next(in)
			for _, critic := range []retryladder.Rung{retryladder.R0, retryladder.R1, retryladder.R2} {
				in.Critic = critic
				got := retryladder.Next(in)
				if got.CriticRung != critic {
					t.Errorf("%s at %s with critic %s: CriticRung = %q", class, rung, critic, got.CriticRung)
				}
				got.CriticRung = without.CriticRung
				if got.Action != without.Action || got.Rung != without.Rung || got.Class != without.Class || got.Target != without.Target || !slices.Equal(got.Signals, without.Signals) {
					t.Errorf("%s at %s: the critic arm %s changed the decision: %+v vs %+v", class, rung, critic, got, without)
				}
			}
			if without.CriticRung != "" {
				t.Errorf("%s at %s: no critic given, CriticRung = %q, want empty", class, rung, without.CriticRung)
			}
		}
	}
}

func TestBounds_DefaultsAreDeepAndHigh(t *testing.T) {
	if retryladder.DefaultMaxTier != "deep" || retryladder.DefaultMaxEffort != "high" {
		t.Fatalf("defaults = %q, %q, want deep, high", retryladder.DefaultMaxTier, retryladder.DefaultMaxEffort)
	}
	got := retryladder.Bounds{}.Resolved()
	if got.MaxTier != "deep" || got.MaxEffort != "high" {
		t.Errorf("Bounds{}.Resolved() = %+v, want deep and high", got)
	}
	set := retryladder.Bounds{MaxTier: "top", MaxEffort: "xhigh"}.Resolved()
	if set.MaxTier != "top" || set.MaxEffort != "xhigh" {
		t.Errorf("set bounds resolved to %+v, want them kept", set)
	}
	half := retryladder.Bounds{MaxTier: "top"}.Resolved()
	if half.MaxTier != "top" || half.MaxEffort != "high" {
		t.Errorf("one bound set: Resolved() = %+v, want top and the default high", half)
	}
}

func tierIndex(tier string) int          { return slices.Index(modelcatalog.CanonicalTiers, tier) }
func effortIndex(e string) int           { return slices.Index(policy.EffortLevels(), e) }
func pick[T any](r *rand.Rand, xs []T) T { return xs[r.Intn(len(xs))] }

func TestNext_PropertyNoInputBeatsTheBounds(t *testing.T) {
	r := rand.New(rand.NewSource(1977))
	classes := []retryladder.Class{retryladder.Infra, retryladder.Quota, retryladder.Halt, retryladder.Spec, retryladder.Capability, retryladder.Unknown}
	fingerprints := []string{"codes:a", "codes:b", "codes:c", "text:0123456789ab", ""}
	maxTiers := []string{"", "fast", "balanced", "deep", "top"}
	maxEfforts := []string{"", "low", "medium", "high", "xhigh", "max"}
	efforts := append([]string{""}, policy.EffortLevels()...)
	families := []string{"agy", "agy-claude", "claude"}

	for trial := 0; trial < 3000; trial++ {
		bounds := retryladder.Bounds{MaxTier: pick(r, maxTiers), MaxEffort: pick(r, maxEfforts)}
		resolved := bounds.Resolved()
		tierCap, effortCap := tierIndex(resolved.MaxTier), effortIndex(resolved.MaxEffort)

		baseTier := modelcatalog.CanonicalTiers[r.Intn(tierCap+1)]
		baseEffort := efforts[r.Intn(effortCap+2)]
		if effortIndex(baseEffort) > effortCap {
			baseEffort = ""
		}
		base := convergence.TierEffort{Family: pick(r, families), Tier: baseTier, Model: "m", Effort: baseEffort}

		headroom := convergence.HeadroomTable{}
		for _, family := range families {
			headroom[family] = map[string]convergence.ModelEffort{}
			for _, tier := range modelcatalog.CanonicalTiers {
				if r.Intn(3) > 0 {
					headroom[family][tier] = convergence.ModelEffort{Model: family + tier, Effort: pick(r, efforts)}
				}
			}
		}

		rung := retryladder.R0
		var earlier []string
		raised := 0
		for step := 0; step < 24; step++ {
			fp := pick(r, fingerprints)
			in := retryladder.Input{
				Rung:     rung,
				Earlier:  slices.Clone(earlier),
				Verdict:  retryladder.Verdict{Class: pick(r, classes), Fingerprint: fp},
				Base:     base,
				Chain:    families,
				Headroom: headroom,
				Bounds:   bounds,
				Critic:   pick(r, []retryladder.Rung{"", retryladder.R0, retryladder.R1, retryladder.R2}),
			}
			got := retryladder.Next(in)
			if got.CriticRung != in.Critic {
				t.Fatalf("trial %d: CriticRung %q, want %q", trial, got.CriticRung, in.Critic)
			}
			if got.Action == retryladder.ActionStop {
				break
			}
			if got.Action == retryladder.ActionRetry {
				if got.Rung != rung {
					t.Fatalf("trial %d: a retry moved the rung %s to %s", trial, rung, got.Rung)
				}
				earlier = append(earlier, fp)
				continue
			}
			raised++
			if raised > 3 {
				t.Fatalf("trial %d: %d raised tries from %+v", trial, raised, in)
			}
			if got.Rung <= rung {
				t.Fatalf("trial %d: a raise went from %s to %s", trial, rung, got.Rung)
			}
			if tierIndex(got.Target.Tier) > tierCap {
				t.Fatalf("trial %d: tier %q above max_tier %q", trial, got.Target.Tier, resolved.MaxTier)
			}
			if got.Target.Effort != "" && effortIndex(got.Target.Effort) > effortCap {
				t.Fatalf("trial %d: effort %q above max_effort %q", trial, got.Target.Effort, resolved.MaxEffort)
			}
			if tierIndex(got.Target.Tier) < tierIndex(base.Tier) || effortIndex(got.Target.Effort) < effortIndex(base.Effort) {
				t.Fatalf("trial %d: the raise %+v lowered the base %+v", trial, got.Target, base)
			}
			if got.Rung == retryladder.R2 && tierIndex(got.Target.Tier) != tierIndex(base.Tier)+1 {
				t.Fatalf("trial %d: R2 tier %q is not one step above %q", trial, got.Target.Tier, base.Tier)
			}
			if got.Rung == retryladder.R3 && got.Target.Tier != base.Tier {
				t.Fatalf("trial %d: R3 changed the tier to %q", trial, got.Target.Tier)
			}
			if got.Rung == retryladder.R1 && got.Target.Tier != base.Tier {
				t.Fatalf("trial %d: R1 changed the tier to %q", trial, got.Target.Tier)
			}
			rung = got.Rung
			earlier = append(earlier, fp)
		}
	}
}

func TestNext_UnknownEffortBoundsNeverIndexOutOfRange(t *testing.T) {
	in := capabilityAt(retryladder.R1, "codes:b", "codes:a")
	in.Base = claudeSeat("balanced", "")
	in.Bounds = retryladder.Bounds{MaxEffort: "not-an-effort"}
	got := retryladder.Next(in)
	if got.Action != retryladder.ActionRaise || got.Rung != retryladder.R2 {
		t.Fatalf("Next = %+v, want a raise to R2", got)
	}
	if got.Target.Effort != "" {
		t.Errorf("R2 effort = %q, want empty when no effort bound resolves", got.Target.Effort)
	}
}
