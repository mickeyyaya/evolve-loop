package profiles_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestDeepTierFamilyArrangement(t *testing.T) {
	exceptions := map[string]string{
		"router": "agy-claude-tmux",
	}
	loader, names := profiles.RealTreeProfiles(t)
	floors := policy.DefaultFloors()
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
		if floors.FamilyOf(name) != "" {
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
