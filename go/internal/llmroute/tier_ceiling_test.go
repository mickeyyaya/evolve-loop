package llmroute

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPlanPermits_ACeilingAdmitsOnlyItsFamiliesAtItsTier(t *testing.T) {
	p := Plan{TierCeiling: map[string][]string{"deep": {"claude"}, "top": {"claude"}}}
	cases := []struct {
		cli, tier string
		want      bool
	}{
		{"claude-tmux", "deep", true},
		{"claude-p", "top", true},
		{"agy-tmux", "deep", false},
		{"agy-tmux", "top", false},
		{"agy-tmux", "balanced", true},
		{"agy-tmux", "opus", false},
		{"agy-tmux", "claude-opus-4-1", false},
		{"agy-tmux", "gemini-3.8-flash-high", true},
	}
	for _, tc := range cases {
		if got := p.Permits(tc.cli, tc.tier); got != tc.want {
			t.Errorf("Permits(%s, %s) = %v, want %v", tc.cli, tc.tier, got, tc.want)
		}
	}
	if !(Plan{}).Permits("agy-tmux", "deep") {
		t.Error("a plan with no ceiling permits every attempt")
	}
}

func TestDispatchTiered_ACeilingSkipsAgyAtDeepAndStepsDownToIt(t *testing.T) {
	plan := Plan{
		Candidates:  []string{"agy-tmux", "claude-tmux"},
		Triggers:    []int{85},
		Tiers:       []string{"deep", "balanced"},
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}
	exits := map[string]int{"claude-tmux@deep": 85}
	var stepped []string
	res := DispatchTiered(plan, func(cli, tier string) (int, error) {
		if code := exits[cli+"@"+tier]; code != 0 {
			return code, errors.New("walled")
		}
		return 0, nil
	}, func(from, to string) { stepped = append(stepped, from+">"+to) })
	if want := []string{"claude-tmux@deep", "agy-tmux@balanced"}; !reflect.DeepEqual(res.Attempts, want) {
		t.Fatalf("attempts = %v, want %v", res.Attempts, want)
	}
	if res.CLI != "agy-tmux" || res.Tier != "balanced" || res.Err != nil {
		t.Fatalf("result = %+v, want agy-tmux@balanced success", res)
	}
	if !reflect.DeepEqual(stepped, []string{"deep>balanced"}) {
		t.Fatalf("step-downs = %v", stepped)
	}
}

func TestDispatchTiered_ATierTheCeilingEmptiesStepsDownWithoutALaunch(t *testing.T) {
	plan := Plan{
		Candidates:  []string{"agy-tmux"},
		Triggers:    []int{85},
		Tiers:       []string{"deep", "balanced"},
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}
	res := DispatchTiered(plan, func(cli, tier string) (int, error) { return 0, nil }, nil)
	if !reflect.DeepEqual(res.Attempts, []string{"agy-tmux@balanced"}) || res.Err != nil {
		t.Fatalf("result = %+v, want one agy-tmux@balanced launch", res)
	}
}

func TestDispatchTiered_ACeilingThatPermitsNothingIsASystemFailureNotCapacity(t *testing.T) {
	plan := Plan{
		Candidates:  []string{"agy-tmux"},
		Triggers:    []int{85},
		Tiers:       []string{"deep"},
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}
	launched := false
	res := DispatchTiered(plan, func(cli, tier string) (int, error) { launched = true; return 0, nil }, nil)
	if launched {
		t.Fatal("a ceiling that permits no attempt must launch nothing")
	}
	if !errors.Is(res.Err, ErrNoPermittedAttempt) || res.Walled || len(res.Attempts) != 0 {
		t.Fatalf("result = %+v, want ErrNoPermittedAttempt, no attempt and no known wall", res)
	}
}

func TestDispatchTiered_APermittedBenchedFamilyIsLaunchedSoItsWallIsKnown(t *testing.T) {
	plan := ApplyBench(Plan{
		Candidates:  []string{"claude-tmux", "agy-tmux"},
		Triggers:    []int{85},
		Tiers:       []string{"deep"},
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}, map[string]time.Time{"claude": time.Unix(0, 0)})
	if !reflect.DeepEqual(plan.Candidates, []string{"agy-tmux", "claude-tmux"}) {
		t.Fatalf("the bench demotes claude, never drops it: %v", plan.Candidates)
	}
	res := DispatchTiered(plan, func(cli, tier string) (int, error) { return 85, errors.New("walled") }, nil)
	if !reflect.DeepEqual(res.Attempts, []string{"claude-tmux@deep"}) || !res.Walled || errors.Is(res.Err, ErrNoPermittedAttempt) {
		t.Fatalf("result = %+v, want the benched claude launched at deep and its wall known", res)
	}
}

func TestDispatchTiered_AWalkThatLaunchedReportsItsLastLaunchNotAnEmptiedTier(t *testing.T) {
	plan := Plan{
		Candidates:  []string{"claude-tmux"},
		Triggers:    []int{85},
		Tiers:       []string{"deep", "balanced"},
		TierCeiling: map[string][]string{"balanced": {"agy"}},
	}
	res := DispatchTiered(plan, func(cli, tier string) (int, error) { return 85, errors.New("walled") }, nil)
	if res.CLI != "claude-tmux" || res.Tier != "deep" || !res.Walled {
		t.Fatalf("result = %+v, want the walled claude-tmux@deep launch", res)
	}
}

func TestDefaultDriverForFamily_MapsAFamilyToItsTmuxDriver(t *testing.T) {
	cases := map[string]string{"agy": "agy-tmux", "claude": "claude-tmux", "claude-p": "claude-p", "nope": "nope"}
	for in, want := range cases {
		if got := DefaultDriverForFamily(in); got != want {
			t.Errorf("DefaultDriverForFamily(%s) = %s, want %s", in, got, want)
		}
	}
}
