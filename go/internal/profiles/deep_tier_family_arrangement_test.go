package profiles

import (
	"testing"
)

func TestDeepTierFamilyArrangement(t *testing.T) {
	// The graders' exceptions come from claudeFamilyFloor; only the advisor is pinned here.
	exceptions := map[string]string{
		"router": "agy-tmux",
	}
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
			continue
		}
		if p.CLI != "codex-tmux" {
			t.Errorf("%s: cli=%q, want codex-tmux (deep→codex arrangement, 2026-08-26)", name, p.CLI)
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
