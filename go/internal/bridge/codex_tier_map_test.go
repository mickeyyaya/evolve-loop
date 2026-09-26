package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codexDeepModel is the one value pin for codex's deep and top tiers; every other codex tier test asserts
// relationally against the family manifest. The reasoning rung is pinned in profiles/effort_defaults_test.go.
const codexDeepModel = "gpt-5.6-sol"

// codexFamilyManifest loads the codex family manifest without the live-catalog overlay, so a stale catalog
// cannot mask a manifest regression. codex-tmux.json is the family's one tier table.
func codexFamilyManifest(t *testing.T) Manifest {
	t.Helper()
	m, err := loadManifestRaw("codex-tmux")
	if err != nil {
		t.Fatalf("loadManifestRaw(codex-tmux): %v", err)
	}
	return m
}

// The clamp's safe set must admit the deep model, or the clamp would rewrite every deep launch to its default.
func TestCodexManifest_DeepTopTiers_ValuePin(t *testing.T) {
	m := codexFamilyManifest(t)
	for _, tier := range []string{"deep", "top"} {
		if got := m.ModelTierMap[tier]; got != codexDeepModel {
			t.Errorf("model_tier_map[%s] = %q, want %q (2026-09-10 cost directive)", tier, got, codexDeepModel)
		}
	}
	if m.ChatGPTDefaultModel != codexDeepModel {
		t.Errorf("chatgpt_default_model = %q, want %q", m.ChatGPTDefaultModel, codexDeepModel)
	}
	safe := false
	for _, s := range m.ChatGPTSafeModels {
		if s == codexDeepModel {
			safe = true
		}
	}
	if !safe {
		t.Errorf("chatgpt_safe_models %v must admit %q — else the subscription clamp rewrites deep launches to the default", m.ChatGPTSafeModels, codexDeepModel)
	}
	if m.ModelTierMap["fast"] != "gpt-5.6-luna" || m.ModelTierMap["balanced"] != "gpt-5.6-terra" {
		t.Errorf("fast/balanced rows drifted: %v", m.ModelTierMap)
	}
}

// The pointer is explicit, so a headless manifest that legitimately declares no map (claude-p) is never changed.
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
	r := Realize(fixture, LaunchIntent{ModelTier: "opus"})
	if !containsToken(r.LaunchFlags, "-m") || !containsToken(r.LaunchFlags, "m-deep") {
		t.Errorf("Realize(opus) LaunchFlags = %v, want -m m-deep via the shared ladder", r.LaunchFlags)
	}
	real := codexFamilyManifest(t)
	if a, b := resolveTierModel(real, "opus"), resolveTierModel(real, "deep"); a != b || a != real.ModelTierMap["deep"] {
		t.Errorf("real manifest: opus→%q deep→%q map→%q; all three must agree", a, b, real.ModelTierMap["deep"])
	}
}

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

// The realistic trigger is a corrupt operator override in bridgeManifestDir(): loadManifestRaw does not fall
// back to the embedded copy on a parse error. The CLI default beats a fatal `-m deep` boot.
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

// gpt-6-astra is out of the tier table and the clamp's safe set, so even a stray pin is rewritten to the family default.
func TestCodexFamilyManifest_AstraIsNotSelectable(t *testing.T) {
	m := codexFamilyManifest(t) // raw: a stale live catalog must not mask a manifest regression
	for tier, model := range m.ModelTierMap {
		if model == "gpt-6-astra" {
			t.Errorf("tier %s still maps to gpt-6-astra", tier)
		}
	}
	for _, s := range m.ChatGPTSafeModels {
		if s == "gpt-6-astra" {
			t.Errorf("chatgpt_safe_models still admits gpt-6-astra — the clamp must rewrite a stray pin to it")
		}
	}
}
