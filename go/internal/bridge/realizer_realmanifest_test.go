package bridge

import (
	"reflect"
	"testing"
)

// realizer_realmanifest_test.go — RealizeFor against the REAL embedded
// manifests (not constructed fixtures). This is the contract the cycle-1 boot
// failure violated: the SAME intent must realize to each CLI's own launch
// flags and never leak one CLI's vocabulary into another. Flags-first: model
// is a launch flag for claude (--model), codex (-m), and agy (-m — corrected
// 2026-05; agy's model_tier was wrongly channel=noop). agy's model_tier_map
// pins every tier to the documented default gemini-3.5-flash for now.

func TestRealizeFor_RealManifests_NoCrossCLILeak(t *testing.T) {
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
		// agy now realizes -m (corrected); the sonnet tier maps via the legacy
		// fallback to balanced → gemini-3.5-flash. Model flag first, then
		// permission; settings_scope stays a no-op for agy.
		if !reflect.DeepEqual(r.LaunchFlags, []string{"-m", "gemini-3.5-flash", "--dangerously-skip-permissions"}) {
			t.Fatalf("agy-tmux = %v, want [-m gemini-3.5-flash --dangerously-skip-permissions]", r.LaunchFlags)
		}
	})

	t.Run("codex-tmux", func(t *testing.T) {
		r := RealizeFor("codex-tmux", intent)
		// codex resolves the tier via its manifest tier_aliases (sonnet→gpt-5.4)
		// and emits it as the -m launch flag (flags-first); no permission flag.
		// Cycle-124 G1a: --yolo from manifest.default_args lands FIRST (defuses
		// the per-edit-approval modal that hung cycle-123 tdd by setting
		// approval=never + sandbox=danger-full-access at boot — undocumented in
		// codex --help 0.134 but parsed by clap; verified empirically). The
		// order is load-bearing: default_args before per-param scalars.
		if !reflect.DeepEqual(r.LaunchFlags, []string{"--yolo", "-m", "gpt-5.4"}) {
			t.Fatalf("codex-tmux = %v, want [--yolo -m gpt-5.4]", r.LaunchFlags)
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
