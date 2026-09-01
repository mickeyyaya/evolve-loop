package profiles

// deep_tier_family_arrangement_test.go — the 2026-08-26 operator directive:
// deep/top-tier task types run on codex (gpt-5.6-sol at the directed rung — high since 2026-09-01; effort is pinned by effort_defaults_test.go, not here), EXCEPT the two
// adversarial checks whose independence from the codex builder is the
// pipeline's anti-gaming core (cross-family floor: builder=codex ⇒ its graders
// are another family) and the advisor brain. Pins the WHOLE arrangement so a
// single-profile drift — either direction — is loud: a mover slipping back to
// claude silently sheds sol leverage; auditor/adversarial-review slipping to
// codex silently puts codex in judgment of codex.

import (
	"testing"
)

func TestDeepTierFamilyArrangement(t *testing.T) {
	// The adversarial exceptions PROJECT from claudeFamilyFloor (the one home
	// of "which phases stay off the builder's family and why" —
	// family_floor_test.go); only the advisor brain is pinned here directly.
	exceptions := map[string]string{
		"router": "agy-tmux", // advisor brain — separate decision
	}
	// TrackedRealProfileNames is the package's ONE funnel over the live
	// profiles dir: the runtime mints untracked stubs into the same directory,
	// and a raw ReadDir scanner reds on state no CI checkout can see (the
	// 2026-08-09 zero-ship batch, fingerprint cd49274beab2) — exactly the
	// shape this test's first draft reintroduced.
	loader, names := RealTreeProfiles(t)
	checked := 0
	for _, name := range names {
		p, gerr := loader.Get(name)
		if gerr != nil {
			continue
		}
		if p.ModelTierDefault != "deep" && p.ModelTierDefault != "top" {
			continue
		}
		checked++
		if want, ok := exceptions[name]; ok {
			if p.CLI != want {
				t.Errorf("%s: cli=%q, want %q — the advisor exception is load-bearing", name, p.CLI, want)
			}
			continue
		}
		if _, floored := claudeFamilyFloor[name]; floored {
			// Family correctness (vs the live builder) is asserted by
			// TestClaudeFamilyFloor's reverse direction — one home, projected.
			continue
		}
		if p.CLI != "codex-tmux" {
			t.Errorf("%s: cli=%q, want codex-tmux (deep→gpt-5.6-sol arrangement, 2026-08-26)", name, p.CLI)
		}
		if len(p.CLIFallback) != 1 || p.CLIFallback[0] != "claude-tmux" {
			t.Errorf("%s: cli_fallback=%v, want [claude-tmux] (universal fallback; agy banned)", name, p.CLIFallback)
		}
		_ = p
	}
	if checked < 20 {
		t.Fatalf("only %d deep/top profiles checked — the arrangement guard lost its corpus", checked)
	}
}
