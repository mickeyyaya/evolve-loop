package flagregistry

import "testing"

// TestFlagRegistry_ShellReadFlagsNotDead is the cycle-360 regression guard.
//
// Cycle 360 tried to remove EVOLVE_INNER_SANDBOX / EVOLVE_FORCE_INNER_SANDBOX as
// "dead", trusting a stale 2026-06-11 "no reader on any surface" inventory note —
// but they (and two siblings) have LIVE readers in adapters/claude.sh that drive
// real sandbox-exec wrapping, worktree-aware profiles, and rate-limit recovery.
// The Go-only reader-completeness guard (go/acs/regression/flagreaders) cannot
// see shell readers, so this pins the correct classification until the adapters
// migrate from shell to Go (after which the Go guard covers them automatically).
//
// Each flag must NOT be StatusDead — they are live operator/bridge controls.
func TestFlagRegistry_ShellReadFlagsNotDead(t *testing.T) {
	shellReadFlags := []string{
		"EVOLVE_INNER_SANDBOX",          // claude.sh:342 → SANDBOX_USE → sandbox-exec
		"EVOLVE_FORCE_INNER_SANDBOX",    // claude.sh:323 deprecation bridge → INNER_SANDBOX
		"EVOLVE_PROFILE_WORKTREE_AWARE", // claude.sh:278 worktree-aware profile branch
		"EVOLVE_REINVOKE_CMD",           // claude.sh:624 rate-limit-recovery hint
	}
	for _, name := range shellReadFlags {
		f, ok := Lookup(name)
		if !ok {
			t.Errorf("flag %q missing — it has a live adapters/claude.sh reader; it must keep a registry row (cycle-360)", name)
			continue
		}
		if f.Status == StatusDead {
			t.Errorf("flag %q is StatusDead but has a live shell reader in adapters/claude.sh — "+
				"reclassify active/deprecated (the cycle-360 false-dead class)", name)
		}
	}
}
