package bridge

import (
	"reflect"
	"testing"
)

// realizer_realmanifest_test.go — RealizeFor against the REAL embedded
// manifests (not constructed fixtures). This is the contract the cycle-1 boot
// failure violated: the SAME intent must realize to each CLI's own launch
// flags and never leak one CLI's vocabulary into another. Flags-first: model
// is a launch flag for claude (--model), codex (-m), and — since the
// cycle-447 live probe of agy 1.0.15 — agy (--model, display-name tokens).

func TestRealizeFor_RealManifests_NoCrossCLILeak(t *testing.T) {
	injectCatalogDir(t, t.TempDir()) // pin manifest offline defaults (no host-catalog overlay)
	intent := LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project", SessionMode: "ephemeral"}

	t.Run("claude-tmux", func(t *testing.T) {
		r := RealizeFor("claude-tmux", intent)
		for _, want := range []string{"--dangerously-skip-permissions", "--model", "sonnet", "--setting-sources", "project"} {
			if !containsToken(r.LaunchFlags, want) {
				t.Fatalf("claude-tmux missing %q in %v", want, r.LaunchFlags)
			}
		}
		if containsToken(r.LaunchFlags, "--no-session-persistence") {
			t.Fatalf("claude-tmux must not emit the print-only flag; got %v", r.LaunchFlags)
		}
		if !r.Ephemeral {
			t.Fatal("ephemeral controller hint expected")
		}
	})

	t.Run("agy-tmux", func(t *testing.T) {
		r := RealizeFor("agy-tmux", intent)
		// agy 1.0.15 selects its model via --model (cycle-447 live probe;
		// 1.0.3 had no model flag — incident cycle-154, `-m` is still
		// undefined). Tier "sonnet" resolves via the legacy ladder to
		// balanced → the manifest's offline display-name default. The scalar
		// order (model before permission) is part of the pin; settings_scope
		// stays a no-op for agy.
		want := []string{"--model", "Gemini Flash 3.7 (High)", "--dangerously-skip-permissions"}
		if !reflect.DeepEqual(r.LaunchFlags, want) {
			t.Fatalf("agy-tmux = %v, want %v", r.LaunchFlags, want)
		}
	})

	t.Run("codex-tmux", func(t *testing.T) {
		r := RealizeFor("codex-tmux", intent)
		// codex resolves the tier via its manifest tier map (sonnet→balanced→gpt-5.6-terra)
		// and emits it as the -m launch flag (flags-first); no permission flag.
		// Cycle-124 G1a: --yolo from manifest.default_args lands FIRST (defuses
		// the per-edit-approval modal that hung cycle-123 tdd by setting
		// approval=never + sandbox=danger-full-access at boot — undocumented in
		// codex --help 0.134 but parsed by clap; verified empirically). The
		// order is load-bearing: default_args before per-param scalars.
		// The second -c is plan_mode_reasoning_effort, added 2026-08-27: codex's
		// plan mode does NOT fall back to model_reasoning_effort, so without it
		// entering plan mode silently drops to codex's built-in preset
		// (observed live: gpt-5.6-sol xhigh -> medium). This exact-argv pin is
		// what caught the realizer dropping the repeated -c flag, so keep it
		// exact rather than relaxing it to a Contains check.
		if !reflect.DeepEqual(r.LaunchFlags, []string{"--yolo", "-m", "gpt-5.6-terra", "-c", "model_reasoning_effort=high", "-c", "plan_mode_reasoning_effort=high"}) {
			t.Fatalf("codex-tmux = %v, want [--yolo -m gpt-5.6-terra -c model_reasoning_effort=high -c plan_mode_reasoning_effort=high] (manifest effort default, 2026-08-15 operator directive; plan-mode override 2026-08-27)", r.LaunchFlags)
		}
		if containsToken(r.LaunchFlags, "--dangerously-skip-permissions") {
			t.Fatalf("codex must NOT emit claude's permission flag; trust is handled by --yolo + auto-responder; got %v", r.LaunchFlags)
		}
	})

	t.Run("unknown cli → empty (no-op, never abort)", func(t *testing.T) {
		r := RealizeFor("does-not-exist", intent)
		if len(r.LaunchFlags) != 0 || len(r.REPLInput) != 0 {
			t.Fatalf("unknown cli must realize to nothing; got %+v", r)
		}
	})
}
