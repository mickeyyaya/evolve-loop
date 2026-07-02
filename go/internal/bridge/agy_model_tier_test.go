package bridge

// agy_model_tier_test.go — cycle-447 Task 1 (agy-model-channel-probe-and-wire):
// unit pins for the agy-tmux model_tier channel wired from noop → flag.
//
// Probe evidence (live, 2026-07-02, agy 1.0.15 — the fresh probe incident
// cycle-154 demands): `agy --help` lists `--model` ("Model for the current
// CLI session"); `agy -m X` still errors "flags provided but not defined: -m";
// `agy models` lists 8 display-name tokens matching the live catalog's
// `available` byte-for-byte; a tmux launch `agy --model "Claude Opus 4.6
// (Thinking)" --dangerously-skip-permissions` boots to the "? for shortcuts"
// footer in ~2s with the model shown in banner + footer. Transcript in
// cycle-447 build-report.md.
//
// The tokens are display names with spaces and parens, so launchCmdLine
// shell-quotes every realized flag token (safe tokens pass through verbatim —
// claude/codex/ollama launch lines stay byte-identical).

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// loadAgyManifestOffline loads the real embedded agy-tmux manifest with the
// live-catalog overlay pinned to an empty directory, so assertions see the
// manifest's OWN offline defaults (Task 1C: sane behavior when the catalog is
// absent/stale).
func loadAgyManifestOffline(t *testing.T) Manifest {
	t.Helper()
	injectCatalogDir(t, t.TempDir())
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	return m
}

// TestAgyModelTierDeepRealizesModelFlag pins the wired channel end-to-end at
// the Realize seam: tier=deep emits --model plus the manifest's deep display
// name, and the offline tier map offers >= 2 distinct models so tier choice
// stays meaningful without a live catalog.
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

// TestAgyModelTierResolvesThroughCatalogOverlay pins the overlay path: a LIVE
// catalog entry for agy overrides the manifest default, and Realize emits the
// live pick (the catalog stays SSOT — the manifest map is only the offline
// fallback the offline test above covers).
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
	// Tiers the catalog didn't carry keep the manifest's offline default.
	if overlaid.ModelTierMap["fast"] != m.ModelTierMap["fast"] {
		t.Fatalf("fast tier must keep the manifest default; got %v", overlaid.ModelTierMap)
	}
}

// TestAgyModelTierAutoSentinelOmitted pins the cycle-262 guard through the
// NEWLY-wired channel: "auto" is the loop's resolve-me sentinel, never a
// concrete model — tier=auto must emit no --model pair, no REPL input, and
// must never leak the literal "auto" into the launch flags.
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

// TestAgyModelTierUnknownTierPassthrough pins the matrix-wide identity
// fallback on the agy channel: a value that is neither a canonical tier nor a
// legacy alias is treated as a RAW model identifier and passes through
// verbatim (same semantics claude/codex already have), and an empty tier
// emits nothing.
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

// TestAgyLaunchCmdLineQuotesDisplayNameModel pins the shared-seam quoting:
// a display-name token (spaces + parens) is POSIX-single-quoted into the one
// shell line SendKeys delivers, while safe-charset tokens — every claude/
// codex/ollama flag — pass through verbatim (byte-identical constraint).
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
