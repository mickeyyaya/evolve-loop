package bridge

import "testing"

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
