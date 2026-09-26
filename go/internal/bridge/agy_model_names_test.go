package bridge

import (
	"regexp"
	"strings"
	"testing"
)

// agyModelName is the shape agy accepts: a family name, then an effort parenthetical. agy's picker
// shows the family alone, so a name captured from it is incomplete.
var agyModelName = regexp.MustCompile(`^[A-Za-z0-9.\- ]+ \((Low|Medium|High|Thinking)\)$`)

func TestAgyManifestTierModelsAreNamesAgyAccepts(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if len(m.ModelTierMap) == 0 {
		t.Fatal("agy-tmux declares no model_tier_map — this guard would be vacuous")
	}
	for tier, model := range m.ModelTierMap {
		if !agyModelName.MatchString(model) {
			t.Errorf("agy tier %q model %q is not a name agy accepts — it needs the capability/effort parenthetical, e.g. %q. agy does NOT error on a bad name; it silently serves Gemini 3.5 Flash (Medium) for the whole session.",
				tier, model, "Gemini 3.7 Flash (Low)")
		}
		// A transposed "Gemini Flash 3.7" passes the shape rule, so pin the word order.
		if strings.Contains(model, "Flash") && !strings.Contains(model, "Flash (") {
			t.Errorf("agy tier %q model %q puts the version AFTER \"Flash\"; agy spells it \"Gemini <version> Flash (<effort>)\"", tier, model)
		}
	}
}

// Each name was launched against agy 1.1.22 and resolved to itself.
func TestAgyTierModelsPinnedToVerifiedNames(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	for tier, want := range map[string]string{
		"fast":     "Gemini 3.7 Flash (Low)",
		"balanced": "Gemini 3.7 Flash (High)",
		"deep":     "Gemini 3.1 Pro (High)",
		"top":      "Gemini 3.1 Pro (High)",
	} {
		if got := m.ModelTierMap[tier]; got != want {
			t.Errorf("agy tier %q = %q, want %q (verified accepted live on agy 1.1.22)", tier, got, want)
		}
	}
}
