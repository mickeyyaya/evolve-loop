package bridge

// agy_model_names_test.go — every agy tier model must be a name agy actually
// ACCEPTS, not merely a plausible one.
//
// Live incident 2026-08-28. Two independent spellings had drifted, and neither
// failed loudly:
//
//	manifest model_tier_map : "Gemini Flash 3.7 (Low)"   <- words transposed
//	live catalog tier_models: "Gemini 3.7 Flash"         <- suffix missing
//
// agy rejects both. It does not exit non-zero; it prints one warning and then
// serves a different model for the whole session:
//
//	⎿ model Gemini 3.1 Pro is not recognized as a known model or custom model
//	  in settings. Using "Gemini 3.5 Flash (Medium)" instead.
//
// Because catalog_overlay merges the live catalog OVER the manifest, the
// catalog's spelling won, so router and memo ran Gemini 3.5 Flash (Medium) at
// EVERY tier from 2026-08-14 until this fix. Nothing detected it: the flag was
// emitted correctly, the launch succeeded, and every flag-shape assertion in
// the suite stayed green. Only the running process knew.
//
// Verified live against agy 1.1.22 (`agy --model "<name>"`, reading the banner
// the process prints about ITSELF):
//
//	"Gemini 3.7 Flash (Low)"     -> Gemini 3.7 Flash (Low)      accepted
//	"Gemini 3.7 Flash (High)"    -> Gemini 3.7 Flash (High)     accepted
//	"Gemini 3.1 Pro (High)"      -> Gemini 3.1 Pro (High)       accepted
//	"Gemini 3.7 Flash"           -> Gemini 3.5 Flash (Medium)   REJECTED
//	"Gemini Flash 3.7 (Low)"     -> Gemini 3.5 Flash (Medium)   REJECTED

import (
	"regexp"
	"strings"
	"testing"
)

// agyModelName is the shape agy accepts: a family name followed by a
// parenthetical capability/effort marker. agy's picker shows the family alone
// and puts effort on a separate slider, so a name captured from that pane is
// structurally incomplete — this is the rule that catches such a name before
// it reaches a launch.
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
		// The 2026-08-14 transposition specifically: "Gemini Flash 3.7"
		// instead of "Gemini 3.7 Flash". Both match the shape rule above, so
		// the shape rule alone would NOT have caught it — pin the word order.
		if strings.Contains(model, "Flash") && !strings.Contains(model, "Flash (") {
			t.Errorf("agy tier %q model %q puts the version AFTER \"Flash\"; agy spells it \"Gemini <version> Flash (<effort>)\"", tier, model)
		}
	}
}

// The exact tier assignment, pinned so a rename cannot quietly re-point a tier
// at a weaker model. These four strings were each launched against agy 1.1.22
// and confirmed to resolve to themselves.
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
