package bridge

import "testing"

// TestModelFreshness_ClaudeDeclaresAlias: claude-tmux declares the alias
// freshness fact — the claude CLI resolves a bare family alias ("opus") to
// that family's newest release at LAUNCH (verified live 2026-07-27:
// --model opus → canonicalModel claude-opus-5), so the alias is strictly
// fresher than any concrete id a catalog could cache. This is a fact about
// the claude binary, declared as manifest DATA beside model_tier_map — the
// same category as chatgpt_safe_models, and zero "claude" conditionals in Go.
func TestModelFreshness_ClaudeDeclaresAlias(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(claude-tmux): %v", err)
	}
	if m.ModelFreshness.Prefer != "alias" {
		t.Errorf("Prefer = %q, want alias", m.ModelFreshness.Prefer)
	}
	wantAliases := map[string]bool{"opus": false, "sonnet": false, "haiku": false}
	for _, id := range m.ModelFreshness.AliasIDs {
		if _, known := wantAliases[id]; known {
			wantAliases[id] = true
		}
	}
	for id, seen := range wantAliases {
		if !seen {
			t.Errorf("AliasIDs missing %q (got %v)", id, m.ModelFreshness.AliasIDs)
		}
	}
	// Every alias id must be a model the tier map can actually emit — an
	// alias the CLI never dispatches is a stale declaration.
	tierModels := make(map[string]bool, len(m.ModelTierMap))
	for _, model := range m.ModelTierMap {
		tierModels[model] = true
	}
	for _, id := range m.ModelFreshness.AliasIDs {
		if !tierModels[id] {
			t.Errorf("alias %q is not a model_tier_map value — declaration drifted from the tier map", id)
		}
	}
}

// TestModelFreshness_AbsentIsZeroValue: every other manifest omits the block
// and gets the zero value — the enumerating-CLI default (newest concrete
// version). Operator manifests stay byte-identical.
func TestModelFreshness_AbsentIsZeroValue(t *testing.T) {
	t.Parallel()
	for _, name := range ManifestNames() {
		if name == "claude-tmux" {
			continue
		}
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if m.ModelFreshness.Prefer != "" || len(m.ModelFreshness.AliasIDs) != 0 {
			t.Errorf("%s: ModelFreshness = %#v, want zero value", name, m.ModelFreshness)
		}
	}
}
