//go:build acs

// Package cycle1029 materialises the cycle-1029 acceptance criteria for this
// fleet lane's sole inbox item, loop-skill-goal-mandatory-prompt-before-act
// (triage top_n slug: loop-skill-goal-mandatory-prompt).
//
// The defect is a DOC/BINARY DIVERGENCE, not a binary defect. The Go binary
// already requires a goal (go/cmd/evolve/cmd_loop_args.go:151-156 → rc=10
// "a goal is required …", locked by dispatch_test.go's
// TestDispatch_LoopRoutesToRunLoop). But skills/loop/SKILL.md still presents
// the goal as OPTIONAL (`[goal]` in the argument-hint and Usage lines), its
// STRICT MODE section has no rule telling the handler to prompt-and-wait for a
// goal before dispatch, and its dispatcher-exit table maps rc=10 only to a
// generic "Bad arguments" with no goal re-prompt. A first-time user running
// bare `/evo:loop` therefore gets a raw rc=10 CLI error instead of a guided
// prompt. The fix is docs + a STRICT MODE handler rule + a durable Go
// regression test; the binary itself must NOT change (Scout: "Do NOT change
// cmd_loop_args.go's goal-required gate").
//
// Predicate strategy. The deliverable of this task IS the SKILL.md wording —
// the criterion is the documentation text itself, not a proxy for a code path
// (contrast the cycle-85 degenerate-predicate ban, which forbids grepping
// PRODUCTION SOURCE for a magic string as a stand-in for behaviour). Predicates
// 001-003 assert on the emitted SKILL.md doc artifact, and each carries the
// `// acs-predicate: config-check` waiver because an inherent documentation-text
// criterion has no runtime code path to exercise (the same waiver cycle-943 used
// for its inherent doc-comment criterion). 001 additionally asserts that the
// DURABLE Go regression test (AC-1's core deliverable) physically exists in
// go/cmd/evolve and references both SKILL.md and the goal-required wording, so a
// doc-only edit that skips the permanent lock fails it.
//
// SKILL.md is a SOURCE doc that Builder edits in the worktree; per the ACS
// dual-root convention the source root is the worktree, reached via
// acsassert.RepoRoot (git toplevel), matching cycle-354's read of
// docs/architecture/control-flags.md.
//
// RED today (all three fail by assertion, not compile):
//   - 001: SKILL.md:4 argument-hint and SKILL.md:160 Usage still contain the
//     `[goal]` optional bracket and no `<goal>` required marker; and no durable
//     regression test in go/cmd/evolve references the SKILL.md goal wording yet.
//   - 002: the STRICT MODE section has no prompt-and-wait-for-goal rule.
//   - 003: the rc=10 table row reads "Bad arguments | Re-prompt with valid args"
//     with no goal re-prompt.
package cycle1029

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// skillRelPath is the doc under test — the /evo:loop skill definition.
const skillRelPath = "skills/loop/SKILL.md"

// cmdEvolveRelPath holds the durable regression test AC-1 requires. Scout's
// targetFiles allow a new file OR extending an existing test in this package;
// docs_contract_test.go (its findRepoRoot helper) is the natural home.
const cmdEvolveRelPath = "go/cmd/evolve"

// readSkill returns the full SKILL.md text and its lines, failing (not
// skipping) when the doc is unreadable — its absence is a task failure.
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

// firstLineWith returns the first line containing every needle, or "" if none.
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

// strictModeSection returns the body between the "## STRICT MODE" heading and
// the next "## " heading (the section whose handler rule AC-2 targets).
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
// TestC1029_001_ArgumentHintAndUsageRequireGoal materialises AC-1: SKILL.md's
// argument-hint AND Usage lines must mark the goal REQUIRED (no `[goal]`
// optional bracket, a `<goal>` required marker present), and a DURABLE Go
// regression test in go/cmd/evolve must lock that wording. Inherent
// documentation-text + test-deliverable-presence criterion → config-check
// waiver (no runtime code path to exercise; the SKILL.md text IS the fix).
func TestC1029_001_ArgumentHintAndUsageRequireGoal(t *testing.T) {
	_, lines := readSkill(t)

	// The argument-hint frontmatter line (SKILL.md:4).
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

	// The Usage line (SKILL.md:160).
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

	// The durable regression test (AC-1's permanent lock) must exist in
	// go/cmd/evolve and reference both the SKILL doc and its goal wording, so a
	// doc-only edit that skips the permanent regression guard fails here.
	if !cmdEvolveTestLocksSkillGoalWording(t) {
		t.Errorf("no durable Go regression test in %s references %s and its goal-required wording;\n"+
			"AC-1 requires a permanent test that fails if a future edit re-introduces `[goal]`.",
			cmdEvolveRelPath, skillRelPath)
	}
}

// cmdEvolveTestLocksSkillGoalWording reports whether some *_test.go under
// go/cmd/evolve both names the SKILL doc and asserts on the goal-required
// wording (a `[goal]`/`<goal>`/argument-hint token) — evidence the durable
// regression lock was delivered, not just the doc edited.
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
		// The optional/required goal bracket is the discriminator: only a test
		// that actually locks the goal wording contains `[goal]` or `<goal>`.
		// `argument-hint` alone is too weak (unrelated skill-publish tests embed
		// a fake argument-hint frontmatter line, cycle-1029 false-positive).
		assertsGoalWording := strings.Contains(src, "[goal]") ||
			strings.Contains(src, "<goal>")
		if namesSkill && assertsGoalWording {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
// TestC1029_002_StrictModePromptAndWaitRule materialises AC-2: the STRICT MODE
// section must carry an explicit imperative rule that the handler PROMPT the
// user for a goal (and WAIT) when none is supplied, and must NOT dispatch a
// goal-less `evolve loop`. Inherent documentation-text criterion → config-check
// waiver.
func TestC1029_002_StrictModePromptAndWaitRule(t *testing.T) {
	_, lines := readSkill(t)
	section := strings.ToLower(strictModeSection(lines))
	if strings.TrimSpace(section) == "" {
		t.Fatalf("could not locate the `## STRICT MODE` section in %s", skillRelPath)
	}

	// The rule must (a) be about a goal, (b) instruct asking/prompting the user,
	// (c) instruct waiting for the answer, and (d) forbid dispatching goal-less.
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
// TestC1029_003_Rc10RowMapsToGoalReprompt materialises AC-3: the dispatcher-exit
// table row for rc=10 must map to a user re-prompt FOR THE GOAL, not a bare
// "Bad arguments". Inherent documentation-text criterion → config-check waiver.
func TestC1029_003_Rc10RowMapsToGoalReprompt(t *testing.T) {
	_, lines := readSkill(t)

	// The rc=10 table row is a Markdown table line (starts with `|`) whose exit
	// cell is `10`. Match the backtick-quoted exit code used in the table.
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
