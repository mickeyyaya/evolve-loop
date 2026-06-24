// cmd_loop_skill_budget_test.go guards a product decision: the cost-budget
// feature (--budget-usd / --budget / --batch-cap-usd) is removed — per-cycle
// token cost is tracked accurately across LLM CLIs as display-only telemetry,
// not exposed as a cap parameter — so no user-facing surface may advertise it.
// This pins the loop skill doc; the removed CLI flags are stripped-with-WARN,
// covered by cmd_loop_test.go and budget_flags_test.go.
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
