package cliroute_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func compileSynthetic(t *testing.T, block policy.CLIRouting, opts ...cliroute.CompileOption) []cliroute.Finding {
	t.Helper()
	_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t), opts...)
	return findings
}

func TestCompile_AbsentBlockIsTheLegacyProjectionWithNoFindings(t *testing.T) {
	_, findings := cliroute.Compile(policy.Policy{Pins: map[string]policy.Pin{"build": {CLI: "claude"}}}, syntheticCatalog(), syntheticProfiles(t))
	if len(findings) != 0 {
		t.Fatalf("an absent block compiles the legacy projection with no findings, got %+v", findings)
	}
}

func TestCompile_ClaudeMissingFromClisIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"agy"}, Default: []string{"agy"}})
	requireFinding(t, findings, "cli_routing.clis", cliroute.SeverityError, "claude")
}

func TestCompile_FloorAgentAssignedOffClaudeIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Agents: map[string]policy.AgentRule{"auditor": {CLI: []string{"agy", "claude"}}},
	})
	requireFinding(t, findings, "cli_routing.agents.auditor", cliroute.SeverityError, "agy-tmux", "[claude]")
}

func TestCompile_APhaseKeyNormalizesToItsAgent(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Agents: map[string]policy.AgentRule{"audit": {CLI: []string{"agy"}}, "build": {CLI: []string{"claude"}}},
	})
	requireFinding(t, findings, "cli_routing.agents.audit", cliroute.SeverityError, "auditor")
	if _, ok := findingFor(findings, "cli_routing.agents.build", cliroute.SeverityError); ok {
		t.Fatalf("agents.build names the builder's phase and is valid: %+v", findings)
	}
}

func TestCompile_TwoKeysNamingOneAgentIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"claude"}, Default: []string{"claude"},
		Agents: map[string]policy.AgentRule{"audit": {CLI: []string{"claude"}}, "auditor": {CLI: []string{"claude"}}},
	})
	requireFinding(t, findings, "cli_routing.agents.auditor", cliroute.SeverityError, "audit")
}

func TestCompile_CrossFamilyCollapseIsAnErrorWithTwoFamilies(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"claude", "agy"}})
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityError, "claude")
}

func TestCompile_CrossFamilyCollapseIsAWarnOnAClaudeOnlyInstall(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}})
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn, "claude")
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("a Claude-only install has no second family to split onto: %+v", errs)
	}
}

func TestCompile_AFallbackSharingAFamilyIsAWarn(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}})
	if errs := errorFindings(findings); len(errs) > 0 {
		t.Fatalf("different primaries are not an error: %+v", errs)
	}
	requireFinding(t, findings, "cross_family_with.auditor+builder", cliroute.SeverityWarn, "claude")
	requireFinding(t, findings, "cross_family_with.builder+tdd-engineer", cliroute.SeverityWarn, "claude")
}

func TestCompile_UnknownAgentRoleTierFamilyAndModelAreFindings(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs:    []string{"claude", "gemini"},
		Default: []string{"claude", "openrouter"},
		Work:    map[string][]string{"review": {"claude"}},
		Agents:  map[string]policy.AgentRule{"nobody": {CLI: []string{"claude"}}, "scout": {CLI: []string{"claude"}, Model: "giant"}},
		Tiers:   map[string]policy.TierRule{"huge": {CLIs: []string{"claude"}}},
	})
	requireFinding(t, findings, "cli_routing.clis", cliroute.SeverityError, "gemini")
	requireFinding(t, findings, "cli_routing.default", cliroute.SeverityError, "openrouter")
	requireFinding(t, findings, "cli_routing.work.review", cliroute.SeverityError, "plan", "build", "evaluate", "control")
	requireFinding(t, findings, "cli_routing.agents.nobody", cliroute.SeverityError, "profile", "phase")
	requireFinding(t, findings, "cli_routing.agents.scout.model", cliroute.SeverityError, "giant")
	requireFinding(t, findings, "cli_routing.tiers.huge", cliroute.SeverityError, "fast", "top")
}

func TestCompile_ADriverThatIsNotToolCapableIsAFinding(t *testing.T) {
	toolless := cliroute.WithToolCapable(func(driver string) bool { return driver != "ollama-tmux" })
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"claude", "ollama"}, Default: []string{"claude"}}, toolless)
	requireFinding(t, findings, "cli_routing.clis", cliroute.SeverityError, "ollama-tmux", "tool")
}

func TestCompile_ATierEntryOutsideClisIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude", "codex"}}, "top": {CLIs: []string{}}},
	})
	requireFinding(t, findings, "cli_routing.tiers.deep", cliroute.SeverityError, "codex")
	requireFinding(t, findings, "cli_routing.tiers.top", cliroute.SeverityError, "empty")
}

func TestCompile_AnUnknownAfterChainIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}, AfterChain: "retry"})
	requireFinding(t, findings, "cli_routing.after_chain", cliroute.SeverityError, "other_clis", "stop")
}

func TestCompile_AnEmptyChainIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{},
		Work:   map[string][]string{"evaluate": {"agy"}},
		Agents: map[string]policy.AgentRule{"memo": {CLI: nil}},
	})
	requireFinding(t, findings, "cli_routing.default", cliroute.SeverityError, "empty")
	requireFinding(t, findings, "cli_routing.agents.memo", cliroute.SeverityError, "empty")
	requireFinding(t, findings, "agent.auditor", cliroute.SeverityError, "work:evaluate")
}

func TestCompile_TwoSourcesAreFindings(t *testing.T) {
	exclude := []string{}
	p := routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}})
	p.Pins = map[string]policy.Pin{"build": {CLI: "claude"}}
	enabled := true
	p.Workflow = &policy.WorkflowPolicy{UniversalFallback: &enabled, UniversalFallbackExclude: exclude}
	p.Router = &policy.RouterPolicy{CLI: "claude", Model: "deep"}
	_, findings := cliroute.Compile(p, syntheticCatalog(), syntheticProfiles(t))
	for _, key := range []string{"pins", "workflow.universal_fallback", "workflow.universal_fallback_exclude", "router.cli", "router.model"} {
		requireFinding(t, findings, key, cliroute.SeverityError, "evolve cli-routing migrate")
	}
}

func TestCompile_ReportsEveryFinding(t *testing.T) {
	p := routingPolicy(policy.CLIRouting{
		CLIs:       []string{"agy", "nope"},
		Default:    []string{"agy"},
		Work:       map[string][]string{"review": {"agy"}},
		Agents:     map[string]policy.AgentRule{"auditor": {CLI: []string{"agy"}}, "ghost": {CLI: []string{"agy"}}},
		Tiers:      map[string]policy.TierRule{"deep": {CLIs: []string{"agy"}}, "colossal": {CLIs: []string{"agy"}}},
		AfterChain: "sometimes",
	})
	p.Pins = map[string]policy.Pin{"scout": {Model: "fast"}}
	_, findings := cliroute.Compile(p, syntheticCatalog(), syntheticProfiles(t))
	for _, key := range []string{
		"cli_routing.clis", "cli_routing.work.review", "cli_routing.agents.auditor", "cli_routing.agents.ghost",
		"cli_routing.tiers.colossal", "cli_routing.after_chain", "pins", "agent.tdd-engineer", "cli_routing.tiers.deep",
	} {
		if _, ok := findingFor(findings, key, cliroute.SeverityError); !ok {
			t.Errorf("missing finding %s in %+v", key, findings)
		}
	}
}

func TestCompile_TierCeilingExcludingClaudeAtAFloorTierIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"agy"}}},
	})
	requireFinding(t, findings, "cli_routing.tiers.deep", cliroute.SeverityError, "auditor", "claude")
}

func TestCompile_AnAgentModelTierChecksTheFloorCeiling(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Agents: map[string]policy.AgentRule{"tdd-engineer": {CLI: []string{"claude"}, Model: "top"}},
		Tiers:  map[string]policy.TierRule{"top": {CLIs: []string{"agy"}}},
	})
	requireFinding(t, findings, "cli_routing.tiers.top", cliroute.SeverityError, "tdd-engineer")
}

func TestCompile_AnUnknownKeyInTheBlockIsRefusedAtLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"cli_routing": {"clis": ["claude"], "defaults": ["claude"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Load(path); err == nil || !strings.Contains(err.Error(), "defaults") {
		t.Fatalf("an unknown key in the block must refuse the load, got %v", err)
	}
}

type failingProfiles struct{}

func (failingProfiles) Get(string) (profiles.Profile, error) {
	return profiles.Profile{}, errors.New("unreadable")
}
func (failingProfiles) List() ([]string, error) { return nil, errors.New("disk gone") }

func TestCompile_AnUnreadableProfileListIsAFindingInBothModes(t *testing.T) {
	for name, p := range map[string]policy.Policy{
		"declared": routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}),
		"legacy":   {},
	} {
		_, findings := cliroute.Compile(p, syntheticCatalog(), failingProfiles{})
		if _, ok := findingFor(findings, "profiles", cliroute.SeverityError, "disk gone"); !ok {
			t.Errorf("%s: an unlistable profile source would silently route every agent as profile-less: %+v", name, findings)
		}
	}
}

func TestCompile_AFloorAgentsOverrideTierChecksTheCeiling(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers: map[string]policy.TierRule{"top": {CLIs: []string{"agy"}}},
	})
	requireFinding(t, findings, "cli_routing.tiers.top", cliroute.SeverityError, "spec-verify")
}

func TestCompile_ChecksThePhaselessSelectionOfAnAgentWithSeveralRoles(t *testing.T) {
	cat := syntheticCatalog()
	cat["review"] = phasespec.PhaseSpec{Name: "review", Agent: "evolve-auditor", Role: "plan"}
	_, findings := cliroute.Compile(routingPolicy(policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy"},
		Work: map[string][]string{"evaluate": {"claude"}, "plan": {"claude"}},
	}), cat, syntheticProfiles(t))
	requireFinding(t, findings, "agent.auditor", cliroute.SeverityError, "rule default")
}

func TestCompile_WithoutAProfileSourceIsAFinding(t *testing.T) {
	for _, p := range []policy.Policy{routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}), {}} {
		_, findings := cliroute.Compile(p, syntheticCatalog(), nil)
		requireFinding(t, findings, "profiles", cliroute.SeverityError, "no profile source")
	}
}

func TestCompile_WithoutACatalogABuiltinPhaseKeyStillNormalizes(t *testing.T) {
	_, findings := cliroute.Compile(routingPolicy(policy.CLIRouting{
		CLIs: []string{"claude"}, Default: []string{"claude"},
		Agents: map[string]policy.AgentRule{"audit": {CLI: []string{"claude"}}, "doc-sync": {CLI: []string{"claude"}}},
	}), nil, syntheticProfiles(t))
	if _, ok := findingFor(findings, "cli_routing.agents.audit", cliroute.SeverityError); ok {
		t.Fatalf("audit is a built-in phase whose contract names the auditor: %+v", findings)
	}
	requireFinding(t, findings, "cli_routing.agents.doc-sync", cliroute.SeverityError, "neither")
}

func TestCompile_AnUnknownFamilyInATierCeilingIsAFinding(t *testing.T) {
	block := policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Tiers: map[string]policy.TierRule{"deep": {CLIs: []string{"claude", "gemini"}}},
	}
	_, findings := cliroute.Compile(routingPolicy(block), syntheticCatalog(), syntheticProfiles(t))
	requireFinding(t, findings, "cli_routing.tiers.deep", cliroute.SeverityError, "gemini")
}

func TestCompile_AListedProfileThatDoesNotLoadIsAFinding(t *testing.T) {
	broken := profiles.NewFromFS(fstest.MapFS{
		"x.json": {Data: []byte(`{"name":"x","cli":"claude-tmux","allowed_tools":["$include_policy:missing"]}`)},
	})
	_, findings := cliroute.Compile(routingPolicy(policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}), syntheticCatalog(), broken)
	requireFinding(t, findings, "profiles.x", cliroute.SeverityError, "does not load")
}

func TestCompile_AnAgentServingTwoRolesIsCheckedUnderEach(t *testing.T) {
	cat := syntheticCatalog()
	cat["review"] = phasespec.PhaseSpec{Name: "review", Agent: "evolve-auditor", Role: "plan"}
	_, findings := cliroute.Compile(routingPolicy(policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Work: map[string][]string{"plan": {"agy"}},
	}), cat, syntheticProfiles(t))
	requireFinding(t, findings, "agent.auditor", cliroute.SeverityError, "work:plan")
}

func TestCompile_ATierCeilingRefusalOnAnOverrideTierIsAFinding(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}, AfterChain: "stop",
		Agents: map[string]policy.AgentRule{"scanner": {CLI: []string{"agy"}}},
		Tiers:  map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "balanced": {CLIs: []string{"claude"}}},
	})
	requireFinding(t, findings, "agent.scanner.tier_ceiling", cliroute.SeverityError, "agents:scanner", "[deep balanced]", "[agy-tmux]")
	if _, ok := findingFor(findings, "agent.scanner.tier_ceiling", cliroute.SeverityError, "[fast]"); ok {
		t.Fatalf("scanner's default tier (fast) is uncapped, so only its deep override is refused: %+v", findings)
	}
}

func TestCompile_TheAfterChainTailCanSatisfyTheCeiling(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
		Agents: map[string]policy.AgentRule{"scanner": {CLI: []string{"agy"}}},
		Tiers:  map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "balanced": {CLIs: []string{"claude"}}},
	})
	if _, ok := findingFor(findings, "agent.scanner.tier_ceiling", cliroute.SeverityError); ok {
		t.Fatalf("other_clis appends claude, which the ceiling permits at deep: %+v", findings)
	}
}

func TestCompile_AnAgentModelFixesTheTiersTheCeilingChecks(t *testing.T) {
	findings := compileSynthetic(t, policy.CLIRouting{
		CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}, AfterChain: "stop",
		Agents: map[string]policy.AgentRule{"scanner": {CLI: []string{"agy"}, Model: "fast"}},
		Tiers:  map[string]policy.TierRule{"deep": {CLIs: []string{"claude"}}, "balanced": {CLIs: []string{"claude"}}},
	})
	if _, ok := findingFor(findings, "agent.scanner.tier_ceiling", cliroute.SeverityError); ok {
		t.Fatalf("agents.scanner.model fixes the tier at fast, so the deep override never runs: %+v", findings)
	}
}
