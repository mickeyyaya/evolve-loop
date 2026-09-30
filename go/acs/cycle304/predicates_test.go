//go:build acs

package cycle304

import (
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var declPins = []string{
	"TestCountFromDeclaration",
	"TestCountFallbackToProse",
	"TestFloorDivergenceCorrective",
	"TestReviewer_UsesDeclaredFloors",
	"TestRecorder_DeclaredFloors",
}

var (
	triageOnce sync.Once
	triageOut  string
)

func runTriagePins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	triageOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(declPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./internal/triagecap/")
		triageOut = stdout + "\n" + stderr
	})
	return triageOut
}

func regexpAlternation(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "|"
		}
		out += regexp.QuoteMeta(n)
	}
	return out
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	noTestsRe  = regexp.MustCompile(`(?m)^testing: warning: no tests to run|no test files`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func TestC304_001_DeclarativeFloorCounterPinsPass(t *testing.T) {
	out := runTriagePins(t)
	if noTestsRe.MatchString(out) {
		t.Fatalf("RED: the five declarative-floor-counter pins did not run (not authored / not compiling):\n%s", tail(out, 40))
	}
	if anyFailRe.MatchString(out) {
		t.Fatalf("RED: a declarative-floor-counter pin FAILED — make the declaration-primary path GREEN:\n%s", tail(out, 60))
	}
	for _, name := range declPins {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: pin %s did not emit `--- PASS` (missing/failed/renamed):\n%s", name, tail(out, 60))
		}
	}
}

// acs-predicate: config-check — WAIVED. The committed_floors field is an agent-
func TestC304_002_SchemaAndPersonaDeclareCommittedFloors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	schema := filepath.Join(root, "schemas", "handoff", "triage-decision.schema.json")
	persona := filepath.Join(root, "agents", "evolve-triage.md")

	if !acsassert.FileExists(t, schema) {
		t.Fatalf("RED: %s missing", schema)
	}
	if !acsassert.FileContains(t, schema, "committed_floors") {
		t.Errorf("RED: triage-decision schema does not document committed_floors — the declaration field is ungoverned")
	}
	if !acsassert.FileExists(t, persona) {
		t.Fatalf("RED: %s missing", persona)
	}
	if !acsassert.FileContains(t, persona, "committed_floors") {
		t.Errorf("RED: triage persona does not instruct emitting committed_floors — readers would gate on data the agent never writes")
	}
}

func tail(s string, n int) string {
	lines := splitLines(s)
	if len(lines) <= n {
		return s
	}
	out := ""
	for _, l := range lines[len(lines)-n:] {
		out += l + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
