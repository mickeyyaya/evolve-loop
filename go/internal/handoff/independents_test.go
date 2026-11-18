package handoff_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier(t *testing.T) {
	cases := []struct {
		name   string
		author handoff.Author
		cli    string
		tier   string
		want   bool
	}{
		{"another family at a weaker tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "codex-tmux", "fast", true},
		{"another family at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "codex-tmux", "deep", true},
		{"another family with no tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "agy-tmux", "", true},
		{"same driver at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "deep", false},
		{"same family other driver at the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-p", "deep", false},
		{"same family at a weaker tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "balanced", false},
		{"same family at a stronger tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "top", true},
		{"same family other driver at a stronger tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-p", "top", true},
		{"alias of the author tier is the same tier", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "opus", false},
		{"author alias against the canonical name is the same tier", handoff.Author{CLI: "claude-tmux", Tier: "opus"}, "claude-tmux", "deep", false},
		{"alias of a stronger tier is stronger", handoff.Author{CLI: "claude-tmux", Tier: "sonnet"}, "claude-tmux", "opus", true},
		{"unknown candidate tier is never stronger", handoff.Author{CLI: "claude-tmux", Tier: "deep"}, "claude-tmux", "mystery", false},
		{"bare family name matches its driver", handoff.Author{CLI: "claude", Tier: "deep"}, "claude-tmux", "deep", false},
		{"author tier empty refuses a same-family top seat", handoff.Author{CLI: "claude-tmux", Tier: ""}, "claude-tmux", "top", false},
		{"author tier empty still admits another family", handoff.Author{CLI: "claude-tmux", Tier: ""}, "codex-tmux", "fast", true},
		{"author tier unknown refuses a same-family top seat", handoff.Author{CLI: "claude-tmux", Tier: "mystery"}, "claude-tmux", "top", false},
		{"author cli empty refuses another family", handoff.Author{CLI: "", Tier: "deep"}, "codex-tmux", "top", false},
		{"unattributed author refuses another family", handoff.Author{CLI: "unattributed", Tier: "deep"}, "codex-tmux", "top", false},
		{"unattributed author refuses a stronger tier", handoff.Author{CLI: "unattributed", Tier: "deep"}, "unattributed", "top", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var spec func(cli, tier string) bool = handoff.Independents(c.author)
			if got := spec(c.cli, c.tier); got != c.want {
				t.Fatalf("Independents(%+v)(%q, %q) = %v, want %v", c.author, c.cli, c.tier, got, c.want)
			}
		})
	}
}
