package setup

import (
	"encoding/json"
	"testing"
)

// --- test helpers (synthetic DetectReport literals — Recommend is pure over a
// DetectReport, so these tests need NO filesystem, clock, or network) ---

func famReady(cli string, tm map[string]string) CLIStatus {
	return CLIStatus{CLI: cli, BinaryPresent: true, AuthConfigured: true, Verdict: "ready", TierModels: tm}
}

func famBlocked(cli string) CLIStatus {
	return CLIStatus{CLI: cli, Verdict: "blocked", CapabilityTier: "n/a"}
}

// claudeTM/codexTM mirror tierModelsFor's real maps so model-resolution asserts
// are realistic without loading manifests.
var claudeTM = map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}
var codexTM = map[string]string{"fast": "gpt-5.4-mini", "balanced": "gpt-5.4", "deep": "gpt-5.5"}

// ph builds a profile-sourced PhaseStatus (Default* == Current*, source profile).
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

// 1. Shape: an empty report still yields the three named presets in order, with
// recommended as the default and no available families.
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

// 2. The recommended preset's tier baseline is the PROFILE default tier
// (canonicalized), NOT a hardcoded role→tier table.
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

// 3. A tier above the envelope max is clamped down and flagged.
func TestRecommend_ClampTierToEnvelopeMax(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("x", "claude-tmux", "opus", "fast", "balanced", "balanced", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "x")
	if a.Tier != "balanced" || !a.TierClamped {
		t.Errorf("clamp: got tier=%q clamped=%v, want balanced/true", a.Tier, a.TierClamped)
	}
}

// 4. A phase with no envelope passes the default tier through unchanged.
func TestRecommend_NoEnvelopePassThrough(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("build-planner", "claude-tmux", "sonnet", "", "", "", nil, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "build-planner")
	if a.Tier != "balanced" || a.TierClamped {
		t.Errorf("no-envelope: got tier=%q clamped=%v, want balanced/false", a.Tier, a.TierClamped)
	}
}

// 5. Economy biases one tier-rank down, floored at envelope.min.
func TestRecommend_EconomyBiasesDown(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("a", "claude-tmux", "sonnet", "fast", "balanced", "deep", []string{"all"}, ""),     // floor fast → fast
		ph("b", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""), // floor balanced → balanced
	)
	eco := presetByName(t, Recommend(rep, builtinPresets), "economy")
	if got := asg(t, eco, "a").Tier; got != "fast" {
		t.Errorf("economy a tier = %q, want fast", got)
	}
	if got := asg(t, eco, "b").Tier; got != "balanced" {
		t.Errorf("economy b tier = %q, want balanced (floored at min)", got)
	}
}

// 6. Economy on a fixed (min==max) envelope stays put.
func TestRecommend_EconomyMinEqMaxStays(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("auditor", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "economy"), "auditor").Tier; got != "deep" {
		t.Errorf("economy fixed-envelope tier = %q, want deep", got)
	}
}

// 7. Max-quality biases up to the envelope max.
func TestRecommend_MaxQualityBiasesUp(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "scout").Tier; got != "deep" {
		t.Errorf("max-quality tier = %q, want deep (envelope max)", got)
	}
}

// 8. Max-quality where default==max has no room up; stays.
func TestRecommend_MaxQualityDefaultEqMaxStays(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("tester", "claude-tmux", "sonnet", "balanced", "balanced", "balanced", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "tester").Tier; got != "balanced" {
		t.Errorf("max-quality tier = %q, want balanced", got)
	}
}

// 9. Zero authed families: all presets degraded, every assignment warns, no panic.
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

// 10. Exactly one family: single-family routing is legitimate, NOT a warning.
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

// 11. Two families: builder and auditor are split across families (adversarial),
// preferring each profile's default family when available.
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

// 12. Two families but allow-lists force the same family — no crash, same family.
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

// 13. Preferred CLI unavailable → falls back to an available allowed family.
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

// 14. allowed_clis restricted to an unavailable family → warn + degraded.
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

// 15. allowed_clis ["all"] picks the only available family even if pref differs.
func TestRecommend_AllowedAllPicksAvailable(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("agy", map[string]string{"fast": "gemini-3.5-flash", "balanced": "gemini-3.5-flash", "deep": "gemini-3.5-flash"})},
		ph("intent", "claude-tmux", "opus", "deep", "deep", "deep", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "intent")
	if a.CLI != "agy" || a.Warning != "" {
		t.Errorf("allowed-all: cli=%q warn=%q, want agy/no-warn", a.CLI, a.Warning)
	}
}

// 16. Model id is resolved from the chosen CLI's TierModels at the chosen tier.
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

// 17. Deterministic: same input → byte-identical JSON across runs (guards
// map-iteration nondeterminism in family/CLI selection).
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

// 18b. A profile that omits model_tier_default (empty DefaultTier) must NOT
// spuriously report DiffersFromDefault in the recommended preset — else apply
// would emit a redundant pin. The effective default (envelope.default/balanced)
// is the comparison basis, matching biasTier.
func TestRecommend_EmptyDefaultTier_NoSpuriousDiff(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("x", "claude-tmux", "", "balanced", "balanced", "deep", []string{"all"}, ""),
	)
	a := asg(t, presetByName(t, Recommend(rep, builtinPresets), "recommended"), "x")
	if a.DiffersFromDefault {
		t.Errorf("empty default tier should not spuriously differ in recommended: %+v", a)
	}
}

// 18. DiffersFromDefault: false when the assignment equals the profile default,
// true when a preset moves it.
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
