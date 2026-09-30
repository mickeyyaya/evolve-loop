//go:build acs

package cycle488

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

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

func TestC488_001_CarryoverTodoDropsBoilerplatePrefix(t *testing.T) {
	out, code := runGoTest(t, "TestCarryoverTodoActionDropsBoilerplatePrefix", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover-todo boilerplate-prefix removal is red (exit=%d) — recordFailureLearning still prepends the redundant sentence\n%s", code, out)
	}
}

const maxDefectActionRunes = 600

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

func TestC488_003_WriteCarryoverTodosCapsPerItemLength(t *testing.T) {
	out, code := runGoTest(t, "TestWriteCarryoverTodosCapsPerItemLength", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover-todo render cap is red (exit=%d) — writeCarryoverTodos still renders oversized Action strings in full\n%s", code, out)
	}
}

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
