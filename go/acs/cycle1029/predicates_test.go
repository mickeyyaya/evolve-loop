//go:build acs

package cycle1029

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const skillRelPath = "skills/loop/SKILL.md"

const cmdEvolveRelPath = "go/cmd/evolve"

func readSkill(t *testing.T) (string, []string) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), skillRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", skillRelPath, err)
	}
	body := string(raw)
	return body, strings.Split(body, "\n")
}

func firstLineWith(lines []string, needles ...string) string {
	for _, ln := range lines {
		ok := true
		for _, n := range needles {
			if !strings.Contains(ln, n) {
				ok = false
				break
			}
		}
		if ok {
			return ln
		}
	}
	return ""
}

func strictModeSection(lines []string) string {
	var b strings.Builder
	in := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "## STRICT MODE") {
			in = true
			continue
		}
		if in && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if in {
			b.WriteString(ln)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// acs-predicate: config-check
func TestC1029_001_ArgumentHintAndUsageRequireGoal(t *testing.T) {
	_, lines := readSkill(t)

	hint := firstLineWith(lines, "argument-hint:")
	if hint == "" {
		t.Fatalf("no argument-hint frontmatter line found in %s", skillRelPath)
	}
	if strings.Contains(hint, "[goal]") {
		t.Errorf("argument-hint still frames the goal as OPTIONAL (`[goal]`): %q\n"+
			"AC-1: mark the goal REQUIRED, e.g. `<goal>`.", hint)
	}
	if !strings.Contains(hint, "<goal>") {
		t.Errorf("argument-hint does not mark the goal REQUIRED (`<goal>` absent): %q", hint)
	}

	usage := firstLineWith(lines, "Usage:")
	if usage == "" {
		t.Fatalf("no `Usage:` line found in %s", skillRelPath)
	}
	if strings.Contains(usage, "[goal]") {
		t.Errorf("Usage line still frames the goal as OPTIONAL (`[goal]`): %q\n"+
			"AC-1: mark the goal REQUIRED, e.g. `<goal>`.", usage)
	}
	if !strings.Contains(usage, "<goal>") {
		t.Errorf("Usage line does not mark the goal REQUIRED (`<goal>` absent): %q", usage)
	}

	if !cmdEvolveTestLocksSkillGoalWording(t) {
		t.Errorf("no durable Go regression test in %s references %s and its goal-required wording;\n"+
			"AC-1 requires a permanent test that fails if a future edit re-introduces `[goal]`.",
			cmdEvolveRelPath, skillRelPath)
	}
}

func cmdEvolveTestLocksSkillGoalWording(t *testing.T) bool {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), cmdEvolveRelPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		src := string(raw)
		namesSkill := strings.Contains(src, "SKILL.md") || strings.Contains(src, "skills/loop")
		assertsGoalWording := strings.Contains(src, "[goal]") ||
			strings.Contains(src, "<goal>")
		if namesSkill && assertsGoalWording {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
func TestC1029_002_StrictModePromptAndWaitRule(t *testing.T) {
	_, lines := readSkill(t)
	section := strings.ToLower(strictModeSection(lines))
	if strings.TrimSpace(section) == "" {
		t.Fatalf("could not locate the `## STRICT MODE` section in %s", skillRelPath)
	}

	if !strings.Contains(section, "goal") {
		t.Errorf("STRICT MODE section has no goal-handling rule (no `goal` mention)")
	}
	if !(strings.Contains(section, "prompt") || strings.Contains(section, "ask")) {
		t.Errorf("STRICT MODE section does not instruct the handler to PROMPT/ASK the user for a goal")
	}
	if !strings.Contains(section, "wait") {
		t.Errorf("STRICT MODE section does not instruct the handler to WAIT for the user's goal before dispatch")
	}
	forbidsGoalless := strings.Contains(section, "goal-less") ||
		strings.Contains(section, "goalless") ||
		strings.Contains(section, "without a goal") ||
		strings.Contains(section, "never dispatch") ||
		strings.Contains(section, "not dispatch")
	if !forbidsGoalless {
		t.Errorf("STRICT MODE section does not forbid dispatching a goal-less `evolve loop`\n" +
			"AC-2: it must state the handler must NOT dispatch without a goal.")
	}
}

// acs-predicate: config-check
func TestC1029_003_Rc10RowMapsToGoalReprompt(t *testing.T) {
	_, lines := readSkill(t)

	var row string
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "`10`") {
			row = trimmed
			break
		}
	}
	if row == "" {
		t.Fatalf("no rc=10 (`| `10` | …`) table row found in %s", skillRelPath)
	}
	if !strings.Contains(strings.ToLower(row), "goal") {
		t.Errorf("rc=10 table row does not map to a goal re-prompt: %q\n"+
			"AC-3: rc=10 must guide the user to supply a goal, not just \"Bad arguments\".", row)
	}
}
