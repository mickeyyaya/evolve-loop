package setup

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestTierModelsFor_AgyClaudeReadsTheAgyClaudeTargetsTierMap(t *testing.T) {
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	want := map[string]string{
		"fast":     "Claude Sonnet 5.5 (Low)",
		"balanced": "Claude Sonnet 5.5 (Medium)",
		"deep":     "Claude Opus 5.5 (High)",
		"top":      "Claude Opus 5.5 (High)",
	}
	if got := tierModelsFor("agy-claude"); !reflect.DeepEqual(got, want) {
		t.Fatalf("tierModelsFor(agy-claude) = %v, want %v: a detect fallback that names tiers is no model at all", got, want)
	}
}

func TestCapManifest_AgyClaudeRunsTheAntigravityBinary(t *testing.T) {
	if got := capManifest("agy-claude"); got != "antigravity" {
		t.Fatalf("capManifest(agy-claude) = %q, want antigravity", got)
	}
}

func TestDetectCLIs_ReportsAgyClaudeBesideAgy(t *testing.T) {
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	ready := func(cli string) bridge.DoctorResult {
		return bridge.DoctorResult{
			CLI: cli, Verdict: "ready",
			Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/agy"},
			Auth:   bridge.AuthInfo{Configured: true, SubscriptionType: "google-ai"},
		}
	}
	rep := bridge.DoctorReport{Results: []bridge.DoctorResult{ready("agy-claude-tmux"), ready("agy-tmux"), ready("agy")}}

	clis := detectCLIs(rep, func(base string) string { return capManifest(base) }, func(string) string { return "" })

	if len(clis) != 2 || clis[0].CLI != "agy" || clis[1].CLI != "agy-claude" {
		t.Fatalf("detected %+v, want agy and agy-claude", clis)
	}
	claude := clis[1]
	if claude.Verdict != "ready" || claude.AuthMode != "SUBSCRIPTION" || claude.CapabilityTier != "antigravity" || claude.TierModels["deep"] != "Claude Opus 5.5 (High)" {
		t.Fatalf("agy-claude = %+v, want a ready Antigravity subscription whose deep tier is Claude Opus 5.5 (High)", claude)
	}
}

func TestTierModelsFor_EveryFamilyReadsItsInteractiveTargetsTierMap(t *testing.T) {
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	targets := map[string]string{
		"claude":     "claude-tmux",
		"codex":      "codex-tmux",
		"agy":        "agy-tmux",
		"agy-claude": "agy-claude-tmux",
		"ollama":     "ollama-tmux",
	}
	for family, target := range targets {
		m, err := bridge.LoadManifest(target)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", target, err)
		}
		got := tierModelsFor(family)
		for _, tier := range abstractTiers {
			if got[tier] != m.ModelTierMap[tier] {
				t.Errorf("tierModelsFor(%s)[%s] = %q, want %s's %q", family, tier, got[tier], target, m.ModelTierMap[tier])
			}
		}
	}
}
