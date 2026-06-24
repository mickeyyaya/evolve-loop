// cmd_loop_skill_budget_test.go guards a product decision: the cost-budget
// feature (--budget-usd / --budget) is unsupported — a reliable dollar cost
// can't be derived across different LLMs — so no user-facing surface may
// advertise it. This pins the loop skill doc; the CLI flag itself is a warned
// no-op covered by cmd_loop_test.go.
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
	// Ban the COST-budget flag and its concept terms only — the live cycle-budget
	// feature (advisor-decided termination) legitimately uses the word "budget".
	for _, banned := range []string{"--budget", "budget-usd", "cost-driven", "stop_reason=budget"} {
		if strings.Contains(low, banned) {
			t.Errorf("skills/loop/SKILL.md references the unsupported cost-budget feature (%q); "+
				"the budget flag is removed (cost can't be measured reliably across LLMs) — "+
				"document --cycles N / advisor-decided cycles instead", banned)
		}
	}
}
