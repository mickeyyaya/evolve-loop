// skill_loop_goal_contract_test.go — durable regression lock for the
// /evo:loop goal-required doc contract (cycle-1029, inbox item
// loop-skill-goal-mandatory-prompt-before-act).
//
// The Go binary REQUIRES a goal unless --resume/--dry-run
// (go/cmd/evolve/cmd_loop_args.go:151-156 → rc=10 "a goal is required …",
// locked by dispatch_test.go's TestDispatch_LoopRoutesToRunLoop). This test
// locks the OTHER half of that contract: skills/loop/SKILL.md must frame the
// goal as REQUIRED, so a future edit that re-introduces the optional `[goal]`
// bracket in the argument-hint or Usage line fails the suite here. It is the
// permanent counterpart to the per-cycle ACS predicates in
// go/acs/cycle1029/predicates_test.go, which are pruned after the cycle.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillLoopRelPath is the /evo:loop skill definition under contract.
const skillLoopRelPath = "skills/loop/SKILL.md"

// readSkillLoopLines returns SKILL.md split into lines, failing when the doc
// is unreadable (its absence is a contract failure, not a skip).
func readSkillLoopLines(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(findRepoRoot(t), skillLoopRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", skillLoopRelPath, err)
	}
	return strings.Split(string(raw), "\n")
}

// firstLineContaining returns the first line containing needle, or "".
func firstLineContaining(lines []string, needle string) string {
	for _, ln := range lines {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

// TestSkillLoopGoalRequired_ArgumentHintAndUsage locks the doc/binary parity:
// the argument-hint frontmatter and the Usage line must mark the goal REQUIRED
// (`<goal>`, never the optional `[goal]` bracket). Fails if a future edit
// re-introduces optional-goal framing.
func TestSkillLoopGoalRequired_ArgumentHintAndUsage(t *testing.T) {
	lines := readSkillLoopLines(t)

	hint := firstLineContaining(lines, "argument-hint:")
	if hint == "" {
		t.Fatalf("no argument-hint frontmatter line in %s", skillLoopRelPath)
	}
	if strings.Contains(hint, "[goal]") {
		t.Errorf("argument-hint re-introduced the OPTIONAL goal bracket `[goal]`: %q\n"+
			"the binary requires a goal (rc=10); the doc must mark it `<goal>`.", hint)
	}
	if !strings.Contains(hint, "<goal>") {
		t.Errorf("argument-hint no longer marks the goal REQUIRED (`<goal>` absent): %q", hint)
	}

	usage := firstLineContaining(lines, "Usage:")
	if usage == "" {
		t.Fatalf("no `Usage:` line in %s", skillLoopRelPath)
	}
	if strings.Contains(usage, "[goal]") {
		t.Errorf("Usage line re-introduced the OPTIONAL goal bracket `[goal]`: %q", usage)
	}
	if !strings.Contains(usage, "<goal>") {
		t.Errorf("Usage line no longer marks the goal REQUIRED (`<goal>` absent): %q", usage)
	}
}
