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

func firstLineContaining(lines []string, needle string) string {
	for _, ln := range lines {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

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
