package bridge

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// loadAgyManifestOffline pins the catalog overlay to an empty dir, so assertions see the manifest's own offline defaults.
func loadAgyManifestOffline(t *testing.T) Manifest {
	t.Helper()
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	return m
}

func TestAgyModelTierDeepRealizesModelFlag(t *testing.T) {
	m := loadAgyManifestOffline(t)
	deep := m.ModelTierMap["deep"]
	if deep == "" {
		t.Fatal("agy-tmux model_tier_map has no deep entry")
	}
	r := Realize(m, LaunchIntent{ModelTier: "deep"})
	if !containsToken(r.LaunchFlags, "--model") || !containsToken(r.LaunchFlags, deep) {
		t.Fatalf("Realize(tier=deep) LaunchFlags = %v, want --model %q", r.LaunchFlags, deep)
	}
	distinct := map[string]struct{}{}
	for _, tier := range []string{"fast", "balanced", "deep"} {
		if v := m.ModelTierMap[tier]; v != "" {
			distinct[v] = struct{}{}
		}
	}
	if len(distinct) < 2 {
		t.Fatalf("offline model_tier_map is flat (%v) — need >= 2 distinct models", m.ModelTierMap)
	}
}

func TestAgyModelTierResolvesThroughCatalogOverlay(t *testing.T) {
	m := loadAgyManifestOffline(t)
	cat := modelcatalog.Catalog{
		FetchedAt: time.Now(),
		CLIs: map[string]modelcatalog.CLIEntry{
			"agy": {Source: modelcatalog.SourceLive, TierModels: map[string]string{
				"deep": "Synthetic Deep (Test)",
			}},
		},
	}
	overlaid := applyCatalogTierMap(m, cat)
	r := Realize(overlaid, LaunchIntent{ModelTier: "deep"})
	if !containsToken(r.LaunchFlags, "Synthetic Deep (Test)") {
		t.Fatalf("live-catalog deep pick not realized; LaunchFlags = %v", r.LaunchFlags)
	}
	if overlaid.ModelTierMap["fast"] != m.ModelTierMap["fast"] {
		t.Fatalf("fast tier must keep the manifest default; got %v", overlaid.ModelTierMap)
	}
}

func TestAgyModelTierAutoSentinelOmitted(t *testing.T) {
	m := loadAgyManifestOffline(t)
	r := Realize(m, LaunchIntent{ModelTier: "auto"})
	if containsToken(r.LaunchFlags, "--model") || containsToken(r.LaunchFlags, "auto") {
		t.Fatalf("tier=auto must omit the model param entirely; LaunchFlags = %v", r.LaunchFlags)
	}
	if len(r.REPLInput) != 0 {
		t.Fatalf("tier=auto must not seed REPL input; got %v", r.REPLInput)
	}
}

func TestAgyModelTierUnknownTierPassthrough(t *testing.T) {
	m := loadAgyManifestOffline(t)
	r := Realize(m, LaunchIntent{ModelTier: "Gemini 3.1 Pro (High)"})
	if !containsToken(r.LaunchFlags, "--model") || !containsToken(r.LaunchFlags, "Gemini 3.1 Pro (High)") {
		t.Fatalf("raw model identifier must pass through verbatim; LaunchFlags = %v", r.LaunchFlags)
	}
	empty := Realize(m, LaunchIntent{ModelTier: ""})
	if containsToken(empty.LaunchFlags, "--model") {
		t.Fatalf("empty tier must emit no model flag; LaunchFlags = %v", empty.LaunchFlags)
	}
}

func TestAgyLaunchCmdLineQuotesDisplayNameModel(t *testing.T) {
	got := launchCmdLine("agy", []string{"--dangerously-skip-permissions", "--model", "Gemini 3.1 Pro (High)"})
	want := "agy --dangerously-skip-permissions --model 'Gemini 3.1 Pro (High)'"
	if got != want {
		t.Fatalf("launchCmdLine = %q, want %q", got, want)
	}
	if got := launchCmdLine("claude", []string{"--model", "opus", "--dangerously-skip-permissions"}); got != "claude --model opus --dangerously-skip-permissions" {
		t.Fatalf("safe tokens must join byte-identically; got %q", got)
	}
	if got := launchCmdLine("codex", []string{"--yolo", "-m", "gpt-5.5"}); got != "codex --yolo -m gpt-5.5" {
		t.Fatalf("safe tokens must join byte-identically; got %q", got)
	}
	if !strings.Contains(launchCmdLine("agy", nil), "agy") {
		t.Fatal("empty flags must return the bare binary")
	}
}
