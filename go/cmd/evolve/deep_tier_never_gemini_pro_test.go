package main

import (
	"slices"
	"strings"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func isGeminiPro(model string) bool {
	return modelquery.FamilyOf(model) == "gemini" && strings.Contains(strings.ToLower(model), " pro")
}

func everyBinaryInstalledRouter(t *testing.T) *cliroute.Router {
	t.Helper()
	pol, root := checkedInPolicy(t)
	installed := []gobridge.DoctorResult{
		doctorResult("claude-tmux", true, "ready"), doctorResult("codex-tmux", true, "ready"),
		doctorResult("agy-tmux", true, "ready"), doctorResult("ollama-tmux", true, "ready"),
	}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: pol, Profiles: profiles.NewFromDir(root + "/.evolve/profiles"),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }, Discover: func() []string { return universalFallbackTail(installed) }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func geminiProEntries(t *testing.T, chain []string, tier string) []string {
	t.Helper()
	var hits []string
	for _, driver := range chain {
		m, err := gobridge.LoadManifest(driver)
		if err != nil {
			t.Errorf("chain entry %s has no manifest: %v", driver, err)
			continue
		}
		if model := m.ModelTierMap[tier]; isGeminiPro(model) {
			hits = append(hits, driver+"="+model)
		}
	}
	return hits
}

func resolvedChain(t *testing.T, r *cliroute.Router, req cliroute.Request) []string {
	t.Helper()
	d, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve(%s, launch %q): %v", req.Agent, req.Launch, err)
	}
	return d.Plan.Candidates
}

func permittedChain(t *testing.T, r *cliroute.Router, req cliroute.Request, tier string) []string {
	t.Helper()
	d, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", req.Agent, err)
	}
	var permitted []string
	for _, cli := range d.Plan.Candidates {
		if d.Plan.Permits(cli, tier) {
			permitted = append(permitted, cli)
		}
	}
	return permitted
}

func TestDeepAndTopProfiles_NoChainEntryResolvesTheirTierToAGeminiProModel(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	loader := profiles.NewFromDir(profileDir)
	r := everyBinaryInstalledRouter(t)
	checked := 0
	for _, name := range agents {
		p, err := loader.Get(name)
		if err != nil || (p.ModelTierDefault != "deep" && p.ModelTierDefault != "top") {
			continue
		}
		checked++
		if hits := geminiProEntries(t, slices.Concat([]string{p.CLI}, p.CLIFallback), p.ModelTierDefault); len(hits) > 0 {
			t.Errorf("%s's own chain runs %v at %s: deep and top work runs a Claude model, agy-owned first, and never Gemini Pro (operator rule, 2026-10-06)", name, hits, p.ModelTierDefault)
		}
		chain := permittedChain(t, r, cliroute.Request{Agent: name, Phase: cliroutetest.PhaseOf(name), ProjectRoot: t.TempDir()}, p.ModelTierDefault)
		if hits := geminiProEntries(t, chain, p.ModelTierDefault); len(hits) > 0 {
			t.Errorf("%s resolves to %v at %s, which reaches %v: the cli_routing tiers.%s ceiling must keep Gemini Pro off deep and top work", name, chain, p.ModelTierDefault, hits, p.ModelTierDefault)
		}
	}
	if checked == 0 {
		t.Fatal("no deep or top profile was checked: the guard lost its corpus")
	}
}

func TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	r := everyBinaryInstalledRouter(t)
	for _, req := range []cliroute.Request{
		{Agent: "router", Phase: cliroutetest.PhaseOf("router"), ProjectRoot: t.TempDir()},
		{Agent: "router", Launch: cliroute.LaunchAdvisor, ProjectRoot: t.TempDir()},
	} {
		chain := resolvedChain(t, r, req)
		if want := []string{"agy-claude-tmux", "claude-tmux"}; !slices.Equal(chain, want) {
			t.Errorf("router (launch %q) resolves to %v, want %v: agy-owned Claude, then Claude Code, with no codex and no Gemini tail", req.Launch, chain, want)
		}
	}
}

func TestIsGeminiPro_TellsTheAgyDisplayNamesApart(t *testing.T) {
	if !isGeminiPro("Gemini 3.1 Pro (High)") || isGeminiPro("Gemini 3.8 Flash (High)") || isGeminiPro("Claude Opus 5.5 (High)") {
		t.Fatal("isGeminiPro misreads the agy display names it must tell apart")
	}
}
