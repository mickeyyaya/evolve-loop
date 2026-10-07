//go:build acs

package cycle1821

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var conservativeTriggerSet = []int{80, 81, 85, 124, 127}

type planSource struct {
	name string
	plan func() llmroute.Plan
}

func defaultTriggerSources(prof *profiles.Profile) []planSource {
	return []planSource{
		{"Resolve", func() llmroute.Plan {
			return llmroute.Resolve("scout", "scout", "balanced", nil, prof, nil, nil)
		}},
		{"ChainFor", func() llmroute.Plan { return llmroute.ChainFor("claude-tmux", prof) }},
	}
}

func overwriteAt(t *testing.T, triggers []int, index, value int) {
	t.Helper()
	if index >= len(triggers) {
		t.Fatalf("Triggers=%v has no index %d to edit", triggers, index)
	}
	original := triggers[index]
	triggers[index] = value
	t.Cleanup(func() { triggers[index] = original })
}

func requireConservativeSet(t *testing.T, what string, got []int) {
	t.Helper()
	if !slices.Equal(got, conservativeTriggerSet) {
		t.Errorf("%s: Triggers=%v, want the untouched default %v — an edit to one Plan's Triggers leaked into the package default", what, got, conservativeTriggerSet)
	}
}

func appendThroughReslice(t *testing.T, triggers []int, keep, value int) {
	t.Helper()
	if keep >= len(triggers) {
		t.Fatalf("Triggers=%v too short to reslice at %d", triggers, keep)
	}
	original := triggers[keep]
	grown := append(triggers[:keep], value)
	t.Cleanup(func() { grown[keep] = original })
}

func TestC1821_001_EditingOnePlansTriggersNeverChangesAnotherPlans(t *testing.T) {
	for _, src := range defaultTriggerSources(nil) {
		t.Run(src.name, func(t *testing.T) {
			edited := src.plan()
			requireConservativeSet(t, src.name+" before any edit", edited.Triggers)
			overwriteAt(t, edited.Triggers, 0, 999)
			requireConservativeSet(t, src.name+" after an element write to an earlier plan", src.plan().Triggers)

			appended := src.plan()
			appendThroughReslice(t, appended.Triggers, 2, 4242)
			requireConservativeSet(t, src.name+" after an append through a reslice of an earlier plan", src.plan().Triggers)
			requireConservativeSet(t, "DefaultTriggers after edits to resolved plans", llmroute.DefaultTriggers())

			first, second := src.plan(), src.plan()
			if len(first.Triggers) > 0 && len(second.Triggers) > 0 && &first.Triggers[0] == &second.Triggers[0] {
				t.Errorf("%s: two resolved Plans share one Triggers backing array", src.name)
			}
		})
	}
}

func TestC1821_002_ProfilesWithoutTriggersStillGetAPrivateCopy(t *testing.T) {
	profilesWithoutTriggers := map[string]*profiles.Profile{
		"nil profile":                     nil,
		"profile with nil trigger list":   {CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}},
		"profile with empty trigger list": {CLIFallbackOnExit: []int{}},
		"profile with only a model tier":  {ModelTierDefault: "balanced"},
	}
	for label, prof := range profilesWithoutTriggers {
		for _, src := range defaultTriggerSources(prof) {
			t.Run(label+"/"+src.name, func(t *testing.T) {
				edited := src.plan()
				requireConservativeSet(t, "before any edit", edited.Triggers)
				overwriteAt(t, edited.Triggers, len(edited.Triggers)-1, -1)
				requireConservativeSet(t, "after editing an earlier plan's last trigger", src.plan().Triggers)
			})
		}
	}

	profileTriggers := []int{7, 8}
	configured := &profiles.Profile{CLIFallbackOnExit: profileTriggers}
	for _, src := range defaultTriggerSources(configured) {
		got := src.plan()
		if !slices.Equal(got.Triggers, []int{7, 8}) {
			t.Fatalf("%s: a profile's own trigger list must win, got %v", src.name, got.Triggers)
		}
		overwriteAt(t, got.Triggers, 0, 999)
		if profileTriggers[0] != 7 {
			t.Errorf("%s: editing the Plan rewrote the profile's cli_fallback_on_exit to %v", src.name, profileTriggers)
		}
	}
}

type emptyProfileSource struct{}

func (emptyProfileSource) Get(name string) (profiles.Profile, error) {
	return profiles.Profile{}, fmt.Errorf("no profile %q in the empty source", name)
}

func (emptyProfileSource) List() ([]string, error) { return nil, nil }

func legacyRouter(t *testing.T) *cliroute.Router {
	t.Helper()
	table, findings := cliroute.Compile(policy.Policy{}, nil, emptyProfileSource{})
	everyBinaryPresent := func(bin string) (string, error) { return "/usr/bin/" + bin, nil }
	router, err := cliroute.New(table, cliroute.Host{LookPath: everyBinaryPresent})
	if err != nil {
		t.Fatalf("cliroute.New over a table with findings %v: %v", findings, err)
	}
	return router
}

func TestC1821_003_ProductionRouterDecisionsDoNotShareTriggers(t *testing.T) {
	router := legacyRouter(t)
	launches := map[string]cliroute.Request{
		"dispatch launch (Resolve)": {Agent: "scout", Phase: "scout", DefaultModel: "balanced"},
		"advisor launch (ChainFor)": {Agent: "scout", Phase: "scout", Launch: cliroute.LaunchAdvisor},
	}
	for label, req := range launches {
		t.Run(label, func(t *testing.T) {
			decide := func() cliroute.Decision {
				d, err := router.Resolve(req)
				if err != nil {
					t.Fatalf("Router.Resolve(%+v): %v", req, err)
				}
				if !d.Legacy() {
					t.Fatalf("rule %q: the fixture must exercise the legacy production path", d.Rule)
				}
				return d
			}
			edited := decide()
			requireConservativeSet(t, label+" before any edit", edited.Plan.Triggers)
			overwriteAt(t, edited.Plan.Triggers, 0, 999)
			requireConservativeSet(t, label+" after the caller edited an earlier Decision's Plan", decide().Plan.Triggers)
		})
	}
}
