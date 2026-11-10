package bridge

import (
	"reflect"
	"testing"
)

func TestModelTierPins_ClaudePinsItsDeepAndTopTiersToFable(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	got := m.TierPins()

	want := map[string]string{"deep": "fable", "top": "fable"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TierPins() = %v, want %v", got, want)
	}
	if m.ModelTierMap["balanced"] != "sonnet" || m.ModelTierMap["fast"] != "haiku" {
		t.Errorf("unpinned tiers moved: %v", m.ModelTierMap)
	}
}

func TestDeclaredTierPins_APinWithNoTierMapModelIsDropped(t *testing.T) {
	t.Parallel()
	m := Manifest{
		ModelTierMap:  map[string]string{"deep": "big"},
		ModelTierPins: []string{"deep", "top"},
	}

	got := declaredTierPins(m)

	if want := map[string]string{"deep": "big"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declaredTierPins() = %v, want %v", got, want)
	}
	if pins := declaredTierPins(Manifest{ModelTierMap: map[string]string{"deep": "big"}}); len(pins) != 0 {
		t.Errorf("a manifest with no pins returned %v", pins)
	}
}

func TestUsageModelFor_TheFableWordMapsToThePinnedModel(t *testing.T) {
	t.Parallel()
	model, ok := UsageModelFor("claude-tmux", "Fable")
	if !ok || model != "fable" {
		t.Errorf("UsageModelFor(claude-tmux, Fable) = %q, %v; want fable so an exhausted Fable week benches its seats", model, ok)
	}
}

func TestModelTierPins_EachPinOfEachManifestHasATierMapModel(t *testing.T) {
	t.Parallel()
	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if got, want := len(m.TierPins()), len(m.ModelTierPins); got != want {
			t.Errorf("%s: %d of %d pins have a model in model_tier_map; pins = %v, map = %v", name, got, want, m.ModelTierPins, m.ModelTierMap)
		}
	}
}
