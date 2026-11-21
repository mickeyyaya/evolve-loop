package bridge

import "testing"

func TestAgyHeadlessManifest_PointsAtFamilyTable(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	raw, err := loadManifestRaw("agy")
	if err != nil {
		t.Fatalf("loadManifestRaw(agy): %v", err)
	}
	if len(raw.ModelTierMap) != 0 {
		t.Errorf("agy.json declares model_tier_map %v — the family table lives in agy-tmux.json only", raw.ModelTierMap)
	}
	if raw.ModelTierMapFrom != "agy-tmux" {
		t.Errorf("agy.json model_tier_map_from = %q, want agy-tmux", raw.ModelTierMapFrom)
	}
	family, err := loadManifestRaw("agy-tmux")
	if err != nil {
		t.Fatalf("loadManifestRaw(agy-tmux): %v", err)
	}
	if len(family.ModelTierMap) == 0 {
		t.Fatal("agy-tmux declares no model_tier_map — this guard would be vacuous")
	}
	loaded, err := LoadManifest("agy")
	if err != nil {
		t.Fatalf("LoadManifest(agy): %v", err)
	}
	if len(loaded.ModelTierMap) != len(family.ModelTierMap) {
		t.Fatalf("LoadManifest(agy) tier map %v, want the family table %v", loaded.ModelTierMap, family.ModelTierMap)
	}
	for tier, want := range family.ModelTierMap {
		if got := loaded.ModelTierMap[tier]; got != want {
			t.Errorf("LoadManifest(agy)[%s] = %q, want the family's %q", tier, got, want)
		}
	}
}
