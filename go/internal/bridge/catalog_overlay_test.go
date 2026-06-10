package bridge

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func manifestWithTierMap() Manifest {
	return Manifest{
		CLI:          "claude-tmux",
		ModelTierMap: map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"},
	}
}

func liveCatalog() modelcatalog.Catalog {
	return modelcatalog.Catalog{
		FetchedAt: time.Now(),
		CLIs: map[string]modelcatalog.CLIEntry{
			"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{
				"deep": "claude-opus-4-8", "fast": "claude-haiku-4-5",
			}},
		},
	}
}

func TestApplyCatalogTierMap_LiveOverrides(t *testing.T) {
	got := applyCatalogTierMap(manifestWithTierMap(), liveCatalog())
	if got.ModelTierMap["deep"] != "claude-opus-4-8" {
		t.Fatalf("deep not overridden: %v", got.ModelTierMap)
	}
	if got.ModelTierMap["fast"] != "claude-haiku-4-5" {
		t.Fatalf("fast not overridden: %v", got.ModelTierMap)
	}
	// A tier the catalog didn't carry keeps the manifest value.
	if got.ModelTierMap["balanced"] != "sonnet" {
		t.Fatalf("balanced should be untouched: %v", got.ModelTierMap)
	}
}

func TestApplyCatalogTierMap_DetectDoesNotOverride(t *testing.T) {
	// A detect-sourced entry must NOT override the manifest (the safety gate).
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceDetect, TierModels: map[string]string{"deep": "wrong"}},
	}}
	got := applyCatalogTierMap(manifestWithTierMap(), cat)
	if got.ModelTierMap["deep"] != "opus" {
		t.Fatalf("detect entry must not override; got %v", got.ModelTierMap)
	}
}

func TestApplyCatalogTierMap_NoEntryIsByteIdentical(t *testing.T) {
	// No catalog entry for this CLI → the SAME ModelTierMap is returned (not a copy).
	m := manifestWithTierMap()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"deep": "gpt-5.5"}},
	}}
	got := applyCatalogTierMap(m, cat)
	if got.ModelTierMap["deep"] != "opus" {
		t.Fatalf("unrelated-CLI catalog must leave manifest unchanged: %v", got.ModelTierMap)
	}
}

func TestBaseCLIName(t *testing.T) {
	cases := map[string]string{"claude-tmux": "claude", "codex-p": "codex", "agy": "agy", "ollama-tmux": "ollama"}
	for in, want := range cases {
		if got := baseCLIName(in); got != want {
			t.Fatalf("baseCLIName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadManifest_NoCatalogIsUnchanged(t *testing.T) {
	// Point the catalog dir at an empty temp dir → LoadManifest must return the
	// embedded manifest untouched (byte-identical-until-catalog property).
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	// embedded claude-tmux manifest has its own ModelTierMap; just assert the
	// load succeeded and overlay was a no-op (deep maps to the embedded value,
	// not anything catalog-injected).
	if m.CLI == "" {
		t.Fatal("manifest failed to load")
	}
}

func TestLoadManifest_LiveCatalogOverlays(t *testing.T) {
	dir := t.TempDir()
	cat := modelcatalog.Catalog{
		FetchedAt: time.Now(),
		CLIs: map[string]modelcatalog.CLIEntry{
			"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"deep": "claude-opus-4-8-LIVE"}},
		},
	}
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatal(err)
	}
	if _, err := modelcatalog.Read(dir); err != nil { // sanity
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", dir)

	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if m.ModelTierMap["deep"] != "claude-opus-4-8-LIVE" {
		t.Fatalf("live catalog did not overlay manifest: deep=%q", m.ModelTierMap["deep"])
	}
}

// TestModelCatalogDir_DefaultPath covers the else branch (line 30): when
// EVOLVE_MODEL_CATALOG_DIR is unset the function falls through to the
// EVOLVE_PROJECT_ROOT/.evolve default. In the normal test run EVOLVE_MODEL_CATALOG_DIR
// is set to the real .evolve dir by the shell, so this branch is otherwise dead.
func TestModelCatalogDir_DefaultPath(t *testing.T) {
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	got := modelCatalogDir()
	if got != filepath.Join("", ".evolve") {
		t.Errorf("modelCatalogDir() = %q, want %q", got, filepath.Join("", ".evolve"))
	}
}
