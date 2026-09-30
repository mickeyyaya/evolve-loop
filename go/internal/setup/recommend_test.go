package setup

import (
	"encoding/json"
	"testing"
)

func famReady(cli string, tm map[string]string) CLIStatus {
	return CLIStatus{CLI: cli, BinaryPresent: true, AuthConfigured: true, Verdict: "ready", TierModels: tm}
}

func famBlocked(cli string) CLIStatus {
	return CLIStatus{CLI: cli, Verdict: "blocked", CapabilityTier: "n/a"}
}

var claudeTM = map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}
var codexTM = map[string]string{"fast": "gpt-5.4-mini", "balanced": "gpt-5.4", "deep": "gpt-5.5"}

func ph(role, defCLI, defTier, min, def, max string, allowed []string, crossWith string) PhaseStatus {
	return PhaseStatus{
		Role: role, Source: "profile",
		CurrentCLI: defCLI, CurrentTier: defTier,
		DefaultCLI: defCLI, DefaultTier: defTier,
		Envelope:        Envelope{Min: min, Default: def, Max: max},
		AllowedCLIs:     allowed,
		CrossFamilyWith: crossWith,
	}
}

func mkReport(clis []CLIStatus, phases ...PhaseStatus) DetectReport {
	return DetectReport{CLIs: clis, Phases: phases}
}

func presetByName(t *testing.T, rr RecommendReport, name string) Preset {
	t.Helper()
	for _, p := range rr.Presets {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("preset %q not found in %v", name, rr.Presets)
	return Preset{}
}

func asg(t *testing.T, p Preset, role string) Assignment {
	t.Helper()
	for _, a := range p.Assignments {
		if a.Role == role {
			return a
		}
	}
	t.Fatalf("assignment for role %q not found in preset %q", role, p.Name)
	return Assignment{}
}

func TestRecommend_EmptyReport_ThreePresets(t *testing.T) {
	rr := Recommend(DetectReport{}, builtinPresets)
	if len(rr.Presets) != 3 {
		t.Fatalf("want 3 presets, got %d", len(rr.Presets))
	}
	wantOrder := []string{"recommended", "economy", "max-quality"}
	for i, name := range wantOrder {
		if rr.Presets[i].Name != name {
			t.Errorf("preset[%d] = %q, want %q", i, rr.Presets[i].Name, name)
		}
	}
	if rr.Default != "recommended" {
		t.Errorf("Default = %q, want recommended", rr.Default)
	}
	if len(rr.AvailableFamilies) != 0 || rr.CrossFamilyOK {
		t.Errorf("empty report: families=%v crossOK=%v", rr.AvailableFamilies, rr.CrossFamilyOK)
	}
}

func TestRecommend_RecommendedTierIsProfileDefault(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("scout", "claude-tmux", "sonnet", "fast", "balanced", "deep", []string{"all"}, ""),
		ph("triage", "claude-tmux", "haiku", "fast", "fast", "deep", []string{"all"}, ""),
		ph("auditor", "claude-tmux", "opus", "fast", "deep", "deep", []string{"all"}, ""),
	)
	rec := presetByName(t, Recommend(rep, builtinPresets), "recommended")
	want := map[string]string{"scout": "balanced", "triage": "fast", "auditor": "deep"}
	for role, tier := range want {
		if got := asg(t, rec, role).Tier; got != tier {
			t.Errorf("%s recommended tier = %q, want %q (canon of profile default)", role, got, tier)
		}
	}
}

func TestRecommend_ClampTierToEnvelopeMax(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("x", "claude-tmux", "opus", "fast", "balanced", "balanced", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "x")
	if a.Tier != "balanced" || !a.TierClamped {
		t.Errorf("clamp: got tier=%q clamped=%v, want balanced/true", a.Tier, a.TierClamped)
	}
}

func TestRecommend_NoEnvelopePassThrough(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("build-planner", "claude-tmux", "sonnet", "", "", "", nil, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "build-planner")
	if a.Tier != "balanced" || a.TierClamped {
		t.Errorf("no-envelope: got tier=%q clamped=%v, want balanced/false", a.Tier, a.TierClamped)
	}
}

func TestRecommend_EconomyBiasesDown(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("a", "claude-tmux", "sonnet", "fast", "balanced", "deep", []string{"all"}, ""),
		ph("b", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	eco := presetByName(t, Recommend(rep, builtinPresets), "economy")
	if got := asg(t, eco, "a").Tier; got != "fast" {
		t.Errorf("economy a tier = %q, want fast", got)
	}
	if got := asg(t, eco, "b").Tier; got != "balanced" {
		t.Errorf("economy b tier = %q, want balanced (floored at min)", got)
	}
}

func TestRecommend_EconomyMinEqMaxStays(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "economy"), "auditor").Tier; got != "deep" {
		t.Errorf("economy fixed-envelope tier = %q, want deep", got)
	}
}

func TestRecommend_MaxQualityBiasesUp(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "scout").Tier; got != "deep" {
		t.Errorf("max-quality tier = %q, want deep (envelope max)", got)
	}
}

func TestRecommend_MaxQualityDefaultEqMaxStays(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("tester", "claude-tmux", "sonnet", "balanced", "balanced", "balanced", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "tester").Tier; got != "balanced" {
		t.Errorf("max-quality tier = %q, want balanced", got)
	}
}

func TestRecommend_ZeroFamiliesDegraded(t *testing.T) {
	rep := mkReport([]CLIStatus{famBlocked("claude"), famBlocked("codex")},
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	rr := Recommend(rep, builtinPresets)
	if rr.CrossFamilyOK {
		t.Error("no authed families should not be cross-family-ok")
	}
	if len(rr.Presets) != 3 {
		t.Fatalf("still want 3 presets, got %d", len(rr.Presets))
	}
	for _, p := range rr.Presets {
		if !p.Degraded {
			t.Errorf("preset %q should be degraded when no family authed", p.Name)
		}
		if asg(t, p, "scout").Warning == "" {
			t.Errorf("preset %q scout assignment should carry a warning", p.Name)
		}
	}
}

func TestRecommend_OneFamilySingleFamily(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM), famBlocked("codex")},
		ph("builder", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"claude", "codex"}, "auditor"),
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, "builder"),
	)
	rr := Recommend(rep, builtinPresets)
	if rr.CrossFamilyOK {
		t.Error("one family should not be cross-family-ok")
	}
	rec := presetByName(t, rr, "recommended")
	b, a := asg(t, rec, "builder"), asg(t, rec, "auditor")
	if b.CLI != "claude" || a.CLI != "claude" {
		t.Errorf("single-family: builder=%q auditor=%q, want both claude", b.CLI, a.CLI)
	}
	if b.Warning != "" || a.Warning != "" {
		t.Errorf("single-family is legitimate, not a warning: builder=%q auditor=%q", b.Warning, a.Warning)
	}
	if rec.Degraded {
		t.Error("single-family preset should not be degraded")
	}
}

func TestRecommend_TwoFamiliesCrossFamily(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM), famReady("codex", codexTM)},
		ph("builder", "codex-tmux", "sonnet", "balanced", "balanced", "deep", []string{"claude", "codex"}, "auditor"),
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, "builder"),
	)
	rr := Recommend(rep, builtinPresets)
	if !rr.CrossFamilyOK {
		t.Error("two families should be cross-family-ok")
	}
	rec := presetByName(t, rr, "recommended")
	b, a := asg(t, rec, "builder"), asg(t, rec, "auditor")
	if b.CLI == a.CLI {
		t.Errorf("builder/auditor should differ; both %q", b.CLI)
	}
	if b.CLI != "codex" || a.CLI != "claude" {
		t.Errorf("expected builder=codex auditor=claude (profile defaults), got builder=%q auditor=%q", b.CLI, a.CLI)
	}
}

func TestRecommend_CrossFamilyForcedSame(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM), famReady("codex", codexTM)},
		ph("builder", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"claude"}, "auditor"),
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"claude"}, "builder"),
	)
	rec := presetByName(t, Recommend(rep, builtinPresets), "recommended")
	b, a := asg(t, rec, "builder"), asg(t, rec, "auditor")
	if b.CLI != "claude" || a.CLI != "claude" {
		t.Errorf("forced-same: builder=%q auditor=%q, want both claude", b.CLI, a.CLI)
	}
	if b.Warning != "" || a.Warning != "" {
		t.Errorf("forced-same is allowed+available, not a warning: %q/%q", b.Warning, a.Warning)
	}
}

func TestRecommend_PreferredUnavailableFallsBack(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("codex", codexTM), famBlocked("claude")},
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "scout")
	if a.CLI != "codex" || !a.CLIFallback {
		t.Errorf("fallback: got cli=%q fallback=%v, want codex/true", a.CLI, a.CLIFallback)
	}
	if a.Warning != "" {
		t.Errorf("an available fallback is not a warning, got %q", a.Warning)
	}
}

func TestRecommend_AllowedRestrictedToUnavailableWarns(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("codex", codexTM), famBlocked("claude")},
		ph("tdd-engineer", "claude-tmux", "opus", "deep", "deep", "deep", []string{"claude"}, ""),
	)
	rr := Recommend(rep, builtinPresets)
	a := asg(t, presetByName(t, rr, "recommended"), "tdd-engineer")
	if a.CLI != "claude" || !a.CLIFallback || a.Warning == "" {
		t.Errorf("restricted-unavailable: cli=%q fallback=%v warn=%q, want claude/true/non-empty", a.CLI, a.CLIFallback, a.Warning)
	}
	if !presetByName(t, rr, "recommended").Degraded {
		t.Error("preset with an unsatisfiable phase should be degraded")
	}
}

func TestRecommend_AllowedAllPicksAvailable(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("agy", map[string]string{"fast": "gemini-3.5-flash", "balanced": "gemini-3.5-flash", "deep": "gemini-3.5-flash"})},
		ph("intent", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "intent")
	if a.CLI != "agy" || a.Warning != "" {
		t.Errorf("allowed-all: cli=%q warn=%q, want agy/no-warn", a.CLI, a.Warning)
	}
}

func TestRecommend_ModelFromTierModels(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("codex", codexTM)},
		ph("builder", "codex-tmux", "sonnet", "balanced", "balanced", "deep", []string{"codex"}, ""),
	)
	rr := Recommend(rep, builtinPresets)
	if got := asg(t, presetByName(t, rr, "recommended"), "builder").Model; got != "gpt-5.4" {
		t.Errorf("recommended builder model = %q, want gpt-5.4 (codex balanced)", got)
	}
	if got := asg(t, presetByName(t, rr, "max-quality"), "builder").Model; got != "gpt-5.5" {
		t.Errorf("max-quality builder model = %q, want gpt-5.5 (codex deep)", got)
	}
}

func TestRecommend_Deterministic(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM), famReady("codex", codexTM)},
		ph("builder", "codex-tmux", "sonnet", "balanced", "balanced", "deep", []string{"claude", "codex"}, "auditor"),
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, "builder"),
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	a, _ := json.Marshal(Recommend(rep, builtinPresets))
	b, _ := json.Marshal(Recommend(rep, builtinPresets))
	if string(a) != string(b) {
		t.Errorf("Recommend not deterministic:\n a=%s\n b=%s", a, b)
	}
}

func TestRecommend_CustomPresetConfig_UpAndMin(t *testing.T) {
	cfg := PresetConfig{
		Default: "rich",
		Presets: []PresetSpec{
			{Name: "rich", Description: "one tier richer", TierBias: "up"},
			{Name: "floor", Description: "envelope floor", TierBias: "min"},
		},
	}
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("scout", "claude-tmux", "fast", "fast", "fast", "deep", []string{"all"}, ""),
	)
	rr := Recommend(rep, cfg)
	if rr.Default != "rich" || len(rr.Presets) != 2 {
		t.Fatalf("custom config: default=%q presets=%d", rr.Default, len(rr.Presets))
	}
	if got := asg(t, presetByName(t, rr, "rich"), "scout").Tier; got != "balanced" {
		t.Errorf("up bias: fast → %q, want balanced (one rank richer)", got)
	}
	if got := asg(t, presetByName(t, rr, "floor"), "scout").Tier; got != "fast" {
		t.Errorf("min bias: scout tier = %q, want fast (envelope floor)", got)
	}
}

func TestRecommend_EmptyDefaultTier_NoSpuriousDiff(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("x", "claude-tmux", "", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "x")
	if a.DiffersFromDefault {
		t.Errorf("empty default tier should not spuriously differ in recommended: %+v", a)
	}
}

func TestRecommend_DiffersFromDefault(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("codex", codexTM)},
		ph("builder", "codex-tmux", "sonnet", "balanced", "balanced", "deep", []string{"codex"}, ""),
	)
	rr := Recommend(rep, builtinPresets)
	if asg(t, presetByName(t, rr, "recommended"), "builder").DiffersFromDefault {
		t.Error("recommended == profile default → DiffersFromDefault should be false")
	}
	if !asg(t, presetByName(t, rr, "max-quality"), "builder").DiffersFromDefault {
		t.Error("max-quality upgrades the tier → DiffersFromDefault should be true")
	}
}
