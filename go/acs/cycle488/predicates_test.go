//go:build acs

// Package cycle488 materialises the cycle-488 acceptance criteria.
//
// TRIAGE COMMITTED TWO ## top_n TASKS, both materializable this cycle:
//
//	tighten-carryover-todo-creation-length  (go/internal/core/failure_learning.go)
//	  → C488_001 (drop boilerplate prefix), C488_002 (bound defect length)
//	cap-carryover-todo-render-length        (go/internal/core/phase_advisor.go)
//	  → C488_003 (per-item render cap)
//	both → C488_004 (core+router CI parity, covers the two "tests green" ACs)
//
// Root cause (scout Key Finding "Router/advisor context injection"): the
// `## Carryover todos` section is 7,627 bytes — ~23% of the 33 KB router prompt
// — because two creation paths write unbounded/redundant CarryoverTodo.Action
// strings and the sole render site (writeCarryoverTodos) caps only the COUNT,
// not the per-item length.
//
// Predicate strategy (mirrors cycle480): behavioral predicates EXERCISE the
// system under test — never a source grep (the cycle-85 degenerate-predicate
// trap). C488_002 calls the exported core.ApplyDefectsAsCarryoverTodos directly;
// C488_001/003/004 drive the unexported code paths through the in-package
// go/internal/core tests via `go test` subprocesses and assert exit 0 +
// "tests actually ran" (the cycle480 no-tests-to-run guard).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C488_002 feeds a 5000-rune defect that a no-op renders unbounded;
//	            C488_003's in-package test feeds a 4000-rune Action a no-op
//	            passes through in full.
//	Edge/OOD:   the in-package suite pins len(todos)==0 (empty omit) and the
//	            25-todo ">20 omitted" trailer boundary (regression pins).
//	Semantic:   the creation-time cap (C488_001/002) and the render-time cap
//	            (C488_003) are DISTINCT surfaces — satisfying one must not
//	            silently satisfy the other.
//
// RED strategy (see test-report.md "RED Run Output"): all four predicates are
// RED before the Builder edits failure_learning.go / phase_advisor.go. The
// in-package RED tests (TestApplyDefectsAsCarryoverTodosBoundsLength,
// TestCarryoverTodoActionDropsBoilerplatePrefix,
// TestWriteCarryoverTodosCapsPerItemLength) fail, so every subprocess exits
// non-zero and the direct-call C488_002 assertion trips.
package cycle488

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

// runGoTest runs `go test` on internal package(s) under -race and returns the
// combined output + exit code. Behavioral predicates invoke the system under
// test through its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test exits 0 with "no tests to run", which would green a predicate on
// unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC488_001_CarryoverTodoDropsBoilerplatePrefix (Task1-AC1): a
// recordFailureLearning-created carryover todo no longer carries the redundant
// "Review the failed cycle learning and fix before retrying:" prefix while
// retaining its cycle/phase/error-class info. Drives the failure-learning queue
// path through the in-package test. RED today.
func TestC488_001_CarryoverTodoDropsBoilerplatePrefix(t *testing.T) {
	out, code := runGoTest(t, "TestCarryoverTodoActionDropsBoilerplatePrefix", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover-todo boilerplate-prefix removal is red (exit=%d) — recordFailureLearning still prepends the redundant sentence\n%s", code, out)
	}
}

// maxDefectActionRunes mirrors the in-package RED bound: a 5000-rune defect must
// be bounded well under this ceiling (sibling cap is 500 runes; the Action wraps
// the capped defect in a short prefix).
const maxDefectActionRunes = 600

// TestC488_002_ApplyDefectsBoundsActionLength (Task1-AC2): the exported
// ApplyDefectsAsCarryoverTodos applies the same/equivalent length bound as
// failureLearningSummary. Direct behavioral predicate — feeds a 5000-rune defect
// and asserts every generated Action is bounded. RED today (no cap → ~5026
// runes).
func TestC488_002_ApplyDefectsBoundsActionLength(t *testing.T) {
	huge := strings.Repeat("x", 5000)
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   488,
		Verdict: "FAIL",
		Defects: []string{huge},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)

	if len(state.CarryoverTodos) == 0 {
		t.Fatal("expected at least one carryover todo for a non-blank defect")
	}
	for _, todo := range state.CarryoverTodos {
		if n := len([]rune(todo.Action)); n > maxDefectActionRunes {
			t.Errorf("ApplyDefectsAsCarryoverTodos leaves Action unbounded: got %d runes from a 5000-rune defect, want <= %d", n, maxDefectActionRunes)
		}
	}
}

// TestC488_003_WriteCarryoverTodosCapsPerItemLength (Task2-AC1): the sole prompt
// injection site (writeCarryoverTodos) caps per-item Action length at render
// time. Unexported → driven through the white-box in-package test. RED today.
func TestC488_003_WriteCarryoverTodosCapsPerItemLength(t *testing.T) {
	out, code := runGoTest(t, "TestWriteCarryoverTodosCapsPerItemLength", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover-todo render cap is red (exit=%d) — writeCarryoverTodos still renders oversized Action strings in full\n%s", code, out)
	}
}

// TestC488_004_CoreRouterCIParity (Task1-AC3 + Task2-AC4: core/router tests
// green, no gate/verdict-path behavior changed). Runs the full internal/core
// and internal/router packages under -race (this reruns the empty-omit and
// >20-omitted regression pins too) plus go vet on both. Mirrors the repo-wide CI
// on the touched packages. RED today: the new in-package RED tests fail.
func TestC488_004_CoreRouterCIParity(t *testing.T) {
	out, code := runGoTest(t, "", corePkg, routerPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/core + internal/router is red (exit=%d)\n%s", code, out)
	}
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("go", "vet", corePkg, routerPkg)
	if vetCode != 0 {
		t.Errorf("go vet over internal/core + internal/router is red (exit=%d)\n%s", vetCode, vetOut)
	}
}
