package bridge

import (
	"strings"
	"testing"
)

// codex_plan_effort_test.go — plan mode must inherit the phase's reasoning tier.
//
// Codex's plan mode does NOT fall back to model_reasoning_effort. Its own config
// reference states plan_mode_reasoning_effort is a "plan-mode-specific reasoning
// override" and that "when unset, Plan mode uses its built-in preset default" —
// so the general effort flag the loop already passes is simply ignored the
// moment /plan engages.
//
// Observed live 2026-08-27 on codex-cli 0.147.0, launched exactly as the loop
// launches it (`-m gpt-5.6-sol -c model_reasoning_effort=xhigh`):
//
//	before /plan : "gpt-5.6-sol xhigh · … "
//	after  /plan : "gpt-5.6-sol medium · …  Plan mode"     <- silent downgrade
//
// and with the override added (`-c plan_mode_reasoning_effort=xhigh`):
//
//	after  /plan : "gpt-5.6-sol xhigh · …  Plan mode"      <- tier preserved
//
// The model is unaffected either way: there is no plan-mode model key, and
// `model` applies across all modes.
//
// Why this matters beyond tidiness: every existing parity assertion pins LAUNCH
// FLAGS (realizer_realmanifest_test.go, driver_credentials_test.go), so they all
// stay green while the session actually runs a tier lower. A gate that cannot
// see the downgrade is worse than no gate, because it gets cited as evidence the
// downgrade did not happen. Pinning the flag here is the cheap half; the live
// status-bar reading above is the evidence that the flag does the work.

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
		// The plan-mode override must carry the SAME tier. Plan mode is where
		// the hardest thinking happens, so inheriting a weaker built-in preset
		// is precisely backwards.
		if !strings.Contains(joined, "plan_mode_reasoning_effort="+tier) {
			t.Errorf("effort %q does not set plan_mode_reasoning_effort — entering plan mode will silently drop to codex's built-in preset (observed xhigh -> medium): %v",
				tier, args)
		}
	}
}

// TestCodexPlanEffortMatchesGeneralEffortForEveryTier pins the two keys in
// lockstep. Split values would be worse than either alone: the launch flag would
// claim one tier while the plan phase ran another, which is the exact confusion
// this change exists to remove.
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
	// The manifest default is what an unset phase gets; it must be a real tier
	// so the plan override is never emitted empty.
	if spec.Default == "" {
		t.Error("effort param has no default; an unset phase would emit no effort flags at all")
	}
	if _, ok := spec.Values[spec.Default]; !ok {
		t.Errorf("effort default %q is not one of the declared values %v", spec.Default, spec.Values)
	}
}
