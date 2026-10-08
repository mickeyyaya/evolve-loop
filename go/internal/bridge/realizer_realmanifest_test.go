package bridge

import (
	"reflect"
	"testing"
)

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
		// Tier "sonnet" resolves via the legacy ladder to balanced → the
		// manifest's offline display-name default. The scalar order (model
		// before permission) is part of the pin; settings_scope stays a
		// no-op for agy.
		want := []string{"--model", "Gemini 3.8 Flash (High)", "--dangerously-skip-permissions"}
		if !reflect.DeepEqual(r.LaunchFlags, want) {
			t.Fatalf("agy-tmux = %v, want %v", r.LaunchFlags, want)
		}
	})

	t.Run("codex-tmux", func(t *testing.T) {
		r := RealizeFor("codex-tmux", intent)
		// codex resolves the tier via its manifest tier map and emits it as the
		// -m launch flag; no permission flag. --yolo from manifest.default_args
		// lands FIRST, ahead of the per-param scalars. The second -c is
		// plan_mode_reasoning_effort: codex's plan mode does not fall back to
		// model_reasoning_effort, so without it entering plan mode silently
		// drops to codex's built-in preset. This exact-argv pin is what catches
		// the realizer dropping a repeated -c flag, so keep it exact rather
		// than a Contains check.
		if !reflect.DeepEqual(r.LaunchFlags, []string{"--yolo", "-c", "check_for_update_on_startup=false", "-m", "gpt-5.6-terra"}) {
			t.Fatalf("codex-tmux = %v, want [--yolo -c check_for_update_on_startup=false -m gpt-5.6-terra]: an intent with no effort carries none, because the resolver owns the effort (2026-10-08); update check off, 2026-10-05", r.LaunchFlags)
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
