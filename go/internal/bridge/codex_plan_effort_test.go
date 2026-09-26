package bridge

import (
	"strings"
	"testing"
)

func TestCodexEffortRealizesPlanModeOverride(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	spec, ok := m.Params["effort"]
	if !ok {
		t.Fatal("codex-tmux declares no effort param")
	}
	if len(spec.Values) == 0 {
		t.Fatal("effort param has no values — this guard would be vacuous")
	}
	for tier, args := range spec.Values {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "model_reasoning_effort="+tier) {
			t.Errorf("effort %q does not set model_reasoning_effort: %v", tier, args)
		}
		// Plan mode is where the hardest thinking happens, so it must carry the same tier.
		if !strings.Contains(joined, "plan_mode_reasoning_effort="+tier) {
			t.Errorf("effort %q does not set plan_mode_reasoning_effort — entering plan mode will silently drop to codex's built-in preset (observed xhigh -> medium): %v",
				tier, args)
		}
	}
}

func TestCodexPlanEffortMatchesGeneralEffortForEveryTier(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	spec := m.Params["effort"]
	for tier, args := range spec.Values {
		var general, plan string
		for _, a := range args {
			if v, ok := strings.CutPrefix(a, "model_reasoning_effort="); ok {
				general = v
			}
			if v, ok := strings.CutPrefix(a, "plan_mode_reasoning_effort="); ok {
				plan = v
			}
		}
		if general != plan {
			t.Errorf("tier %q: model_reasoning_effort=%q but plan_mode_reasoning_effort=%q — they must move together",
				tier, general, plan)
		}
	}
	// An unset phase gets the manifest default, so it must be a real tier or the override is emitted empty.
	if spec.Default == "" {
		t.Error("effort param has no default; an unset phase would emit no effort flags at all")
	}
	if _, ok := spec.Values[spec.Default]; !ok {
		t.Errorf("effort default %q is not one of the declared values %v", spec.Default, spec.Values)
	}
}
