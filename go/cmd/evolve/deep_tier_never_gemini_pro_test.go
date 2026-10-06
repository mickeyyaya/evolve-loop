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

var deepTierGeminiProTailAwaitingL2 = map[string]bool{
	"architecture-design": true, "caching-strategy-design": true, "compat-surface-check": true,
	"data-integrity-check": true, "data-model-design": true, "debugger": true, "failure-adjudicator": true,
	"failure-advisor": true, "idempotency-check": true, "intent": true, "merge-to-main-gate": true,
	"migration-safety-check": true, "observability-design": true, "plan-reviewer": true,
	"preliminary-study": true, "premise-challenge": true, "prompt-regression-eval": true,
	"resilience-design": true, "retrospective": true, "rollout-plan": true, "swarm-planner": true,
	"type-safety-audit": true,
}

func resolvedChain(t *testing.T, r *cliroute.Router, req cliroute.Request) []string {
	t.Helper()
	d, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve(%s, launch %q): %v", req.Agent, req.Launch, err)
	}
	return d.Plan.Candidates
}

func TestDeepAndTopProfiles_NoChainEntryResolvesTheirTierToAGeminiProModel(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	profileDir, agents := cliroutetest.TrackedProfiles(t)
	loader := profiles.NewFromDir(profileDir)
	r := everyBinaryInstalledRouter(t)
	checked, awaiting := 0, map[string]bool{}
	for _, name := range agents {
		p, err := loader.Get(name)
		if err != nil || (p.ModelTierDefault != "deep" && p.ModelTierDefault != "top") {
			continue
		}
		checked++
		if hits := geminiProEntries(t, slices.Concat([]string{p.CLI}, p.CLIFallback), p.ModelTierDefault); len(hits) > 0 {
			t.Errorf("%s's own chain runs %v at %s: deep and top work runs a Claude model, agy-owned first, and never Gemini Pro (operator rule, 2026-10-06)", name, hits, p.ModelTierDefault)
		}
		chain := resolvedChain(t, r, cliroute.Request{Agent: name, Phase: cliroutetest.PhaseOf(name), ProjectRoot: t.TempDir()})
		hits := geminiProEntries(t, chain, p.ModelTierDefault)
		switch {
		case len(hits) > 0 && deepTierGeminiProTailAwaitingL2[name]:
			awaiting[name] = true
		case len(hits) > 0:
			t.Errorf("%s resolves to %v at %s, which reaches %v through the universal tail: restrict its allowed_clis, or wait for L2's tiers.deep ceiling and list it in deepTierGeminiProTailAwaitingL2", name, chain, p.ModelTierDefault, hits)
		}
	}
	for name := range deepTierGeminiProTailAwaitingL2 {
		if !awaiting[name] {
			t.Errorf("%s no longer reaches Gemini Pro at deep or top: remove it from deepTierGeminiProTailAwaitingL2", name)
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
