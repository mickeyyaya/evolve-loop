package bridge

import (
	"reflect"
	"testing"
)

// realizer_realmanifest_test.go — RealizeFor against the REAL embedded
// manifests (not constructed fixtures). This is the contract the cycle-1 boot
// failure violated: the SAME intent must realize to each CLI's own launch
// flags and never leak one CLI's vocabulary into another. Flags-first: model
// is a launch flag for claude (--model) and codex (-m), and a no-op for agy
// (no model selector).

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
		if !reflect.DeepEqual(r.LaunchFlags, []string{"--dangerously-skip-permissions"}) {
			t.Fatalf("agy-tmux = %v, want only [--dangerously-skip-permissions] (model/settings are no-ops)", r.LaunchFlags)
		}
	})

	t.Run("codex-tmux", func(t *testing.T) {
		r := RealizeFor("codex-tmux", intent)
		// codex resolves the tier via its manifest tier_aliases (sonnet→gpt-5.4)
		// and emits it as the -m launch flag (flags-first); no permission flag.
		if !reflect.DeepEqual(r.LaunchFlags, []string{"-m", "gpt-5.4"}) {
			t.Fatalf("codex-tmux = %v, want [-m gpt-5.4]", r.LaunchFlags)
		}
		if containsToken(r.LaunchFlags, "--dangerously-skip-permissions") {
			t.Fatalf("codex has no bypass flag; trust is handled by the auto-responder; got %v", r.LaunchFlags)
		}
	})

	t.Run("unknown cli → empty (no-op, never abort)", func(t *testing.T) {
		r := RealizeFor("does-not-exist", intent)
		if len(r.LaunchFlags) != 0 || len(r.REPLInput) != 0 {
			t.Fatalf("unknown cli must realize to nothing; got %+v", r)
		}
	})
}
