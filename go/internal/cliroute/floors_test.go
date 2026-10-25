package cliroute_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func openProfiles() *profiles.Loader {
	return profiles.NewFromFS(fstest.MapFS{
		"spec-verify.json": {Data: []byte(`{"name":"spec-verify","cli":"claude-tmux","model_tier_default":"balanced"}`)},
		"scout.json":       {Data: []byte(`{"name":"scout","cli":"agy-tmux","model_tier_default":"balanced"}`)},
	})
}

func TestCompile_AnEmptyFloorsKeyLetsATableWithoutClaudeRoute(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}, Floors: policy.Floors{}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: openProfiles(), Host: cliroute.Host{LookPath: everyBinary()}})
	d := mustResolve(t, r, cliroute.Request{Agent: "spec-verify"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"agy-tmux"}) || !reflect.DeepEqual(d.Allowed, []string{"agy"}) {
		t.Fatalf("with no floor a former floor agent takes the table's chain: chain=%v allowed=%v", d.Plan.Candidates, d.Allowed)
	}
	if got := r.FloorFamily("spec-verify"); got != "" {
		t.Fatalf("FloorFamily(spec-verify) = %q under an empty floors key", got)
	}
}

func TestCompile_AFloorOnAnotherFamilyPinsItsAgentsThere(t *testing.T) {
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"claude", "agy"}, Floors: policy.Floors{"agy": {"spec-verify"}}}
	r := mustBuild(t, cliroute.Setup{Policy: routingPolicy(block), Profiles: openProfiles(), Host: cliroute.Host{LookPath: everyBinary()}})
	d := mustResolve(t, r, cliroute.Request{Agent: "spec-verify"})
	if !reflect.DeepEqual(d.Plan.Candidates, []string{"agy-tmux"}) || !reflect.DeepEqual(d.Allowed, []string{"agy"}) {
		t.Fatalf("an agy floor keeps only agy: chain=%v allowed=%v", d.Plan.Candidates, d.Allowed)
	}
	if got := r.FloorFamily("spec-verify"); got != "agy" {
		t.Fatalf("FloorFamily(spec-verify) = %q, want agy", got)
	}
	open := mustResolve(t, r, cliroute.Request{Agent: "scout"})
	if !reflect.DeepEqual(open.Allowed, []string{"agy", "claude"}) {
		t.Fatalf("an agent off every floor keeps every listed family: %v", open.Allowed)
	}
}

func TestCompile_EachFloorFamilyMustBeListedAndKnown(t *testing.T) {
	cases := []struct {
		name     string
		floors   policy.Floors
		key      string
		mentions []string
	}{
		{"a floor family outside clis", policy.Floors{"codex": {"auditor"}}, "cli_routing.clis", []string{"codex must be listed", "auditor"}},
		{"an unknown floor family", policy.Floors{"nope": {"auditor"}}, "cli_routing.floors.nope", []string{"unknown family"}},
		{"an agent on two floors", policy.Floors{"agy": {"auditor"}, "claude": {"auditor"}}, "cli_routing.floors.claude", []string{"auditor", "agy"}},
		{"the compiled floor without claude in clis", nil, "cli_routing.clis", []string{"claude must be listed"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			block := policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}, Floors: tc.floors}
			if tc.name == "an agent on two floors" {
				block.CLIs = []string{"agy", "claude"}
			}
			_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t))
			requireFinding(t, findings, tc.key, cliroute.SeverityError, tc.mentions...)
		})
	}
}

func TestCompile_ATierCeilingMustHoldTheFloorFamilyOfItsFloorAgents(t *testing.T) {
	block := policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Floors: policy.Floors{"agy": {"spec-verify"}},
		Tiers:  map[string]policy.TierRule{"balanced": {CLIs: []string{"claude"}}},
	}
	_, findings := cliroute.Compile(routingPolicy(block), nil, openProfiles())
	requireFinding(t, findings, "cli_routing.tiers.balanced", cliroute.SeverityError, "excludes agy", "spec-verify")
}

func TestRouter_FloorFamilyReadsTheCompiledDefaultWithNoBlock(t *testing.T) {
	r := mustBuild(t, cliroute.Setup{Policy: policy.Policy{}, Profiles: syntheticProfiles(t)})
	if got := r.FloorFamily("auditor"); got != "claude" {
		t.Fatalf("FloorFamily(auditor) = %q with no cli_routing block, want the compiled claude floor", got)
	}
	if got := r.FloorFamily("builder"); got != "" {
		t.Fatalf("FloorFamily(builder) = %q, want no floor", got)
	}
}
