package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopSkill_NoBudgetReferences(t *testing.T) {
	root := repoRootForSkills(t)
	raw, err := os.ReadFile(filepath.Join(root, "skills", "loop", "SKILL.md"))
	if err != nil {
		t.Fatalf("read loop SKILL.md: %v", err)
	}
	low := strings.ToLower(string(raw))
	// Bans the cost-budget terms only; the live cycle-budget feature
	// (advisor-decided termination) legitimately uses the word "budget".
	for _, banned := range []string{"--budget", "budget-usd", "cost-driven", "stop_reason=budget"} {
		if strings.Contains(low, banned) {
			t.Errorf("skills/loop/SKILL.md references the unsupported cost-budget feature (%q); "+
				"the budget flag is removed (cost can't be measured reliably across LLMs) — "+
				"document --cycles N / advisor-decided cycles instead", banned)
		}
	}
}
