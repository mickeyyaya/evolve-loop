package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codexAstraDeepModel is the 2026-09-09 operator directive: codex's high
// (deep/top) tiers run gpt-6-astra at the high reasoning rung (the rung is
// pinned by profiles/effort_defaults_test.go — codexDeepTopRung — not here).
// Verified live before the cutover: `codex exec -m gpt-6-astra -c
// model_reasoning_effort=high` on the ChatGPT subscription answered. This is
// the ONE value pin for the directive; every other codex tier test asserts
// relationally against the family manifest.
const codexAstraDeepModel = "gpt-6-astra"

// codexFamilyManifest loads the codex FAMILY manifest raw — WITHOUT the
// live-catalog overlay, so a stale .evolve/model-catalog.json cannot mask a
// manifest regression. codex-tmux.json is the family's single tier table: the
// tmux realization, the headless driver (codex.json points here via model_tier_map_from),
// setup's presets and the advisor clamp probe all resolve through it.
func codexFamilyManifest(t *testing.T) Manifest {
	t.Helper()
	m, err := loadManifestRaw("codex-tmux")
	if err != nil {
		t.Fatalf("loadManifestRaw(codex-tmux): %v", err)
	}
	return m
}

// TestCodexManifest_DeepTopTiers_GPT6Astra pins the directive on the family
// manifest: deep and top resolve to gpt-6-astra, the ChatGPT clamp's default
// is gpt-6-astra, and the clamp's safe set admits it (otherwise the clamp would
// silently rewrite every deep launch back to the default — the cycle-142
// mechanism working against the directive). The fast/balanced rows are
// untouched by the directive.
func TestCodexManifest_DeepTopTiers_GPT6Astra(t *testing.T) {
	m := codexFamilyManifest(t)
	for _, tier := range []string{"deep", "top"} {
		if got := m.ModelTierMap[tier]; got != codexAstraDeepModel {
			t.Errorf("model_tier_map[%s] = %q, want %q (2026-09-09 directive)", tier, got, codexAstraDeepModel)
		}
	}
	if m.ChatGPTDefaultModel != codexAstraDeepModel {
		t.Errorf("chatgpt_default_model = %q, want %q", m.ChatGPTDefaultModel, codexAstraDeepModel)
	}
	safe := false
	for _, s := range m.ChatGPTSafeModels {
		if s == codexAstraDeepModel {
			safe = true
		}
	}
	if !safe {
		t.Errorf("chatgpt_safe_models %v must admit %q — else the subscription clamp rewrites deep launches to the default", m.ChatGPTSafeModels, codexAstraDeepModel)
	}
	if m.ModelTierMap["fast"] != "gpt-5.6-luna" || m.ModelTierMap["balanced"] != "gpt-5.6-terra" {
		t.Errorf("fast/balanced rows drifted: %v", m.ModelTierMap)
	}
}

// TestCodexHeadlessManifest_PointsAtFamilyTable: the family has ONE tier
// table. codex.json (the headless transport) declares no map of its own — its
// copy sat three model generations stale while the driver carried its own
// switch — and instead POINTS at the family manifest (`model_tier_map_from`),
// which LoadManifest resolves. A second copy here would be drift waiting to
// happen; the pointer is explicit so a headless manifest that legitimately
// declares no map (claude-p: the tiers ARE its selectors) is never changed.
func TestCodexHeadlessManifest_PointsAtFamilyTable(t *testing.T) {
	raw, err := loadManifestRaw("codex")
	if err != nil {
		t.Fatalf("loadManifestRaw(codex): %v", err)
	}
	if len(raw.ModelTierMap) != 0 {
		t.Errorf("codex.json declares model_tier_map %v — the family table lives in codex-tmux.json only", raw.ModelTierMap)
	}
	if raw.ModelTierMapFrom != "codex-tmux" {
		t.Errorf("codex.json model_tier_map_from = %q, want codex-tmux", raw.ModelTierMapFrom)
	}
	loaded, err := LoadManifest("codex")
	if err != nil {
		t.Fatalf("LoadManifest(codex): %v", err)
	}
	fam := codexFamilyManifest(t)
	if len(loaded.ModelTierMap) != len(fam.ModelTierMap) {
		t.Fatalf("LoadManifest(codex) tier map %v, want the family table %v", loaded.ModelTierMap, fam.ModelTierMap)
	}
	for tier, want := range fam.ModelTierMap {
		if got := loaded.ModelTierMap[tier]; got != want {
			t.Errorf("LoadManifest(codex)[%s] = %q, want the family's %q", tier, got, want)
		}
	}
}

// TestLoadManifest_ModelTierMapFrom_UnresolvableIsAnError: a pointer that
// names a manifest which is absent or corrupt is a manifest error — loud,
// never an empty map that silently degrades every launch to the CLI default.
func TestLoadManifest_ModelTierMapFrom_UnresolvableIsAnError(t *testing.T) {
	dir := t.TempDir()
	headless := `{"schema_version":1,"cli":"zz-headless","binary":"zz","transport":"headless","model_tier_map_from":"zz-tmux"}`
	if err := os.WriteFile(filepath.Join(dir, "zz-headless.json"), []byte(headless), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, dir)
	if _, err := LoadManifest("zz-headless"); err == nil {
		t.Errorf("LoadManifest with an unresolvable model_tier_map_from must error")
	}
	if err := os.WriteFile(filepath.Join(dir, "zz-tmux.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest("zz-headless"); err == nil {
		t.Errorf("LoadManifest with a corrupt model_tier_map_from target must error")
	}
}

// TestResolveTierModel_FollowsManifest is the single-source proof for inbox
// `codex-tier-map-single-source`: there is ONE tier→model ladder
// (resolveTierModel) and both transports go through it — the realizer's
// flag emit (Realize) and the headless driver's -m composition. Wiring proof:
// a fixture manifest with a different map is followed on both paths with no
// code edit; legacy aliases (haiku/sonnet/opus) resolve through their canonical
// tier; native ids and genuinely unknown values pass through unchanged (the
// cycle-378 contract).
func TestResolveTierModel_FollowsManifest(t *testing.T) {
	fixture := Manifest{
		ModelTierMap: map[string]string{"fast": "m-fast", "balanced": "m-bal", "deep": "m-deep", "top": "m-top"},
		Params:       map[string]ParamSpec{"model_tier": {Channel: "flag", Flag: "-m", From: "model_tier_map"}},
	}
	for _, tc := range []struct{ in, want string }{
		{"fast", "m-fast"}, {"balanced", "m-bal"}, {"deep", "m-deep"}, {"top", "m-top"},
		{"haiku", "m-fast"}, {"sonnet", "m-bal"}, {"opus", "m-deep"},
		{"gpt-x", "gpt-x"}, {"weird", "weird"}, {"", ""},
	} {
		if got := resolveTierModel(fixture, tc.in); got != tc.want {
			t.Errorf("ladder: resolveTierModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// The realizer path emits exactly what the ladder resolves (same function).
	r := Realize(fixture, LaunchIntent{ModelTier: "opus"})
	if !containsToken(r.LaunchFlags, "-m") || !containsToken(r.LaunchFlags, "m-deep") {
		t.Errorf("Realize(opus) LaunchFlags = %v, want -m m-deep via the shared ladder", r.LaunchFlags)
	}
	// Coherence against the REAL family manifest: the legacy alias and the
	// canonical tier resolve identically, to the value the manifest declares.
	real := codexFamilyManifest(t)
	if a, b := resolveTierModel(real, "opus"), resolveTierModel(real, "deep"); a != b || a != real.ModelTierMap["deep"] {
		t.Errorf("real manifest: opus→%q deep→%q map→%q; all three must agree", a, b, real.ModelTierMap["deep"])
	}
}

// TestLaunch_Codex_ResolvesTierViaFamilyManifest: the headless driver's -m is
// the FAMILY manifest's model for the requested tier (relational — the value
// itself is pinned once, above). Both the canonical tier and its legacy alias
// launch the same model.
func TestLaunch_Codex_ResolvesTierViaFamilyManifest(t *testing.T) {
	want := codexFamilyManifest(t).ModelTierMap["deep"]
	for _, tier := range []string{"deep", "opus"} {
		t.Run(tier, func(t *testing.T) {
			fx := newFixture(t, "codex", "")
			fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
			code, _ := runLookup(t, fr, codexArgs(fx, tier), nil)
			if code != ExitOK {
				t.Fatalf("exit = %d, want ExitOK", code)
			}
			if !fr.argvContainsPair("-m", want) {
				t.Fatalf("codex argv should carry -m %s for tier %s; calls=%+v", want, tier, fr.calls)
			}
		})
	}
}

// TestLaunch_Codex_ManifestUnavailable_OmitsModelFlag covers the driver's
// degradation when the family manifest cannot load — the realistic trigger is
// a corrupt operator override in bridgeManifestDir() (`bridge add-rule` writes
// there and loadManifestRaw does NOT fall back to the embedded copy on a parse
// error). The tier then stays untranslated, the vocabulary guard omits -m (the
// CLI default beats a fatal `-m deep` boot — the cycle-378 class), and the
// WARN names the cause.
func TestLaunch_Codex_ManifestUnavailable_OmitsModelFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex-tmux.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, dir)
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, stderr := runLookup(t, fr, codexArgs(fx, "deep"), nil)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (a manifest failure degrades, never aborts); stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "manifest unavailable") {
		t.Errorf("stderr must name the manifest failure; got %q", stderr)
	}
	if len(fr.calls) == 0 {
		t.Fatal("codex was never launched")
	}
	for _, a := range fr.calls[0].args {
		if a == "-m" {
			t.Fatalf("-m must be omitted when the tier cannot be translated; args=%v", fr.calls[0].args)
		}
	}
}
