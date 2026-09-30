//go:build acs

package cycle305

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

var dataPins = []string{
	"TestReadDeferredFloors",
	"TestDeferredFloorPackagesDecl_DeclarationPrimary",
	"TestDeferredFloorPackagesDecl_FallbackToProse",
	"TestDeferredFloorPackagesDecl_CompanionNoFieldFallsBack",
	"TestDeferredFloorPackagesDecl_FiltersToCandidates",
	"TestDeferredFloorDivergence",
	"TestFloorBinding_DeferredFromCompanion",
	"TestFloorBinding_MissingCompanion_FailOpen",
	"TestFloorBinding_ProseIgnoredWithCompanion",
	"TestFloorBinding_CompanionNoField_FallbackProse",
	"TestFloorBinding_DeclaredDivergenceMessage",
}

var guardPins = []string{
	"TestGuardTriageFloors_CleanWorkspaceExitsZero",
	"TestGuardTriageFloors_DivergenceExitsNonZero",
	"TestGuardTriageFloors_HelpIsInformative",
}

var (
	dataOnce  sync.Once
	dataOut   string
	guardOnce sync.Once
	guardOut  string
)

func runDataPins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	dataOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(dataPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./internal/triagecap/", "./internal/evalgate/")
		dataOut = stdout + "\n" + stderr
	})
	return dataOut
}

func runGuardPins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	guardOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(guardPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./cmd/evolve/")
		guardOut = stdout + "\n" + stderr
	})
	return guardOut
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

func assertAllPass(t *testing.T, out string, pins []string, label string) {
	t.Helper()
	if noTestsRe.MatchString(out) {
		t.Fatalf("RED: the %s pins did not run (not authored / not compiling):\n%s", label, tail(out, 40))
	}
	if anyFailRe.MatchString(out) {
		t.Fatalf("RED: a %s pin FAILED — make the declaration-primary path GREEN:\n%s", label, tail(out, 80))
	}
	for _, name := range pins {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: pin %s did not emit `--- PASS` (missing/failed/renamed):\n%s", name, tail(out, 80))
		}
	}
}

func TestC305_001_DeferredDeclarationPinsPass(t *testing.T) {
	assertAllPass(t, runDataPins(t), dataPins, "triagecap+evalgate deferred-declaration")
}

func TestC305_002_TriageFloorsGuardCLIWorks(t *testing.T) {
	assertAllPass(t, runGuardPins(t), guardPins, "triage-floors guard CLI")
}

// acs-predicate: config-check — WAIVED. deferred_floors is an agent-facing contract
func TestC305_003_SchemaAndPersonaDeclareDeferredFloors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	schema := filepath.Join(root, "schemas", "handoff", "triage-decision.schema.json")
	persona := filepath.Join(root, "agents", "evolve-triage.md")

	if !acsassert.FileExists(t, schema) {
		t.Fatalf("RED: %s missing", schema)
	}
	if !acsassert.FileContains(t, schema, "deferred_floors") {
		t.Errorf("RED: triage-decision schema does not document deferred_floors — the declaration field is ungoverned")
	}
	if !acsassert.FileExists(t, persona) {
		t.Fatalf("RED: %s missing", persona)
	}
	if !acsassert.FileContains(t, persona, "deferred_floors") {
		t.Errorf("RED: triage persona does not instruct emitting deferred_floors — the reader would gate on data the agent never writes")
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
