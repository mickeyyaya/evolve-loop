package flagregistry

import "testing"

// TestFlagRegistry_MigratedShellReadFlagsAreDeprecated is the cycle-360
// regression guard, rewritten for the post-migration world.
//
// Cycle 360 tried to remove four flags as "dead", trusting a stale 2026-06-11
// "no reader on any surface" inventory — but at the time they had LIVE readers
// in adapters/claude.sh (sandbox-exec wrapping, worktree-aware profiles,
// rate-limit recovery), so the remediation pinned them as live.
//
// The script→Go migration (TASK A5) then DELETED adapters/claude.sh and the
// other six bash CLI adapters: dispatch now flows through the Go bridge, which
// controls inner-sandbox via EVOLVE_SANDBOX → sandbox.ShouldWrap
// (internal/bridge/sandbox_wrap.go), worktree-awareness via BridgeRequest.Worktree,
// and rate-limit recovery via manifest interactive_prompts. The four flags lost
// their ONLY reader (shell deleted; no Go reader was ever added) and were
// reclassified to StatusDeprecated.
//
// This guard now pins that post-migration invariant. The four flags must:
//   - remain present (operators/docs still reference them; the deprecation
//     bridge keeps EVOLVE_FORCE_INNER_SANDBOX → EVOLVE_INNER_SANDBOX → EVOLVE_SANDBOX
//     meaningful), and
//   - stay StatusDeprecated — they have NO Go reader, so flipping them back to
//     StatusActive would be false (and the Go reader-completeness guard,
//     go/acs/regression/flagreaders, only checks the inverse direction so it
//     would not catch the lie). They must not be silently dropped either.
func TestFlagRegistry_MigratedShellReadFlagsAreDeprecated(t *testing.T) {
	// Former adapters/claude.sh readers, removed in the script→Go migration.
	migratedFlags := []string{
		"EVOLVE_INNER_SANDBOX",          // was claude.sh:342 → SANDBOX_USE → sandbox-exec
		"EVOLVE_FORCE_INNER_SANDBOX",    // was claude.sh:323 deprecation bridge → INNER_SANDBOX
		"EVOLVE_PROFILE_WORKTREE_AWARE", // was claude.sh:278 worktree-aware profile branch
		"EVOLVE_REINVOKE_CMD",           // was claude.sh:624 rate-limit-recovery hint
	}
	for _, name := range migratedFlags {
		f, ok := Lookup(name)
		if !ok {
			t.Errorf("flag %q missing — its shell reader was removed in the script→Go "+
				"migration but it must keep a registry row (deprecation bridge / operator "+
				"reference); do not silently drop it (cycle-360)", name)
			continue
		}
		if f.Status != StatusDeprecated {
			t.Errorf("flag %q has Status %q but must be StatusDeprecated: its only reader "+
				"(adapters/claude.sh) was deleted in the script→Go migration and no Go reader "+
				"exists, so it cannot be Active; the Go bridge supersedes it (cycle-360)",
				name, f.Status)
		}
	}
}
