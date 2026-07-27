package acssuite

// scopelint_test.go — whole-suite meta-predicates are the false-red AMPLIFIER.
// Cycles 1107/1115/1116/1117/1123 each failed on ONE predicate of the shape
// "go test <core+bridge+recovery> stays green": any contamination anywhere in
// those suites (an auditor probe, the shared /tmp/p root, a sibling lane)
// reads as a builder regression. Whole-repo staleness is the regression
// suite's job (195 predicates, every cycle) — a cycle predicate re-sweeping
// packages the cycle never touched is pure duplication with a false-red
// surface. The lint demotes such predicates to SKIP (never RED — a lint
// false-positive must not be able to fail a cycle), loudly, in the verdict.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCyclePredicates(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const outOfScopeSrc = `package cycle9999

import "testing"

// bridgePkg is the const-indirection shape cycle-1117 actually used — the
// lint must resolve package-level consts, not just inline literals.
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func TestC9999_006_suites_stay_green(t *testing.T) {
	_ = []string{"go", "test", "-count=1", "./internal/core/...", bridgePkg}
}

func TestC9999_001_scoped_ok(t *testing.T) {
	_ = []string{"go", "test", "./internal/bridge/..."}
}

func TestC9999_002_own_predicates(t *testing.T) {
	_ = []string{"go", "test", "./acs/cycle9999"}
}
`

// TestLintPredicateScope_FlagsPackagesOutsideTouchedSet pins the core rule:
// a predicate naming a package the cycle never touched is flagged; scoped and
// own-acs-dir references are not.
func TestLintPredicateScope_FlagsPackagesOutsideTouchedSet(t *testing.T) {
	dir := writeCyclePredicates(t, outOfScopeSrc)
	findings, err := LintPredicateScope(dir, []string{"./internal/bridge/..."})
	if err != nil {
		t.Fatalf("LintPredicateScope: %v", err)
	}
	byTest := map[string][]string{}
	for _, f := range findings {
		byTest[f.Test] = append(byTest[f.Test], f.Pattern)
	}
	want := ScopeFinding{Test: "TestC9999_006_suites_stay_green", File: "predicates_test.go", Pattern: "./internal/core/..."}
	if len(findings) != 1 || findings[0] != want {
		t.Errorf("findings = %+v, want exactly [%+v] — bridge is touched (in scope) even via const indirection", findings, want)
	}
	if len(byTest["TestC9999_001_scoped_ok"]) != 0 {
		t.Errorf("scoped predicate flagged: %v — a predicate over a TOUCHED package is exactly what we want authored", byTest["TestC9999_001_scoped_ok"])
	}
	if len(byTest["TestC9999_002_own_predicates"]) != 0 {
		t.Errorf("own acs-dir reference flagged: %v — the cycle's predicate package is always in scope", byTest["TestC9999_002_own_predicates"])
	}
}

// TestLintPredicateScope_ModulePathConstIsInScopeWhenTouched (negative twin of
// the const case): the same const-carried module path must be IN scope when
// its package is touched — resolution must normalize module paths and ./
// patterns to the same key.
func TestLintPredicateScope_ModulePathConstIsInScopeWhenTouched(t *testing.T) {
	dir := writeCyclePredicates(t, outOfScopeSrc)
	findings, err := LintPredicateScope(dir, []string{"./internal/bridge/...", "./internal/core/..."})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none — every referenced package is in the touched set", findings)
	}
}

// TestLintPredicateScope_EmptyTouchedSetLintsNothing: with no handoff there is
// no scope to judge against; the lint must stand down rather than guess (a
// wrong guess here would demote a legitimate gate).
func TestLintPredicateScope_EmptyTouchedSetLintsNothing(t *testing.T) {
	dir := writeCyclePredicates(t, outOfScopeSrc)
	findings, err := LintPredicateScope(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v with an empty touched set — no scope authority, no demotion", findings)
	}
}

// TestLintPredicateScope_IgnoresNonPatternStrings (anti-false-positive): prose,
// file paths, and URLs must never be mistaken for package patterns — a lint
// false-positive demotes a real gate.
func TestLintPredicateScope_IgnoresNonPatternStrings(t *testing.T) {
	dir := writeCyclePredicates(t, `package cycle9999

import "testing"

func TestC9999_003_prose(t *testing.T) {
	_ = "the ./internal-ish prose mention"
	_ = "docs/operations/runtime-reference.md"
	_ = "https://github.com/mickeyyaya/evolve-loop/pull/364"
	_ = "go/internal/bridge/fatalpane.go"
}
`)
	findings, err := LintPredicateScope(dir, []string{"./internal/bridge/..."})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v — none of these strings is a go-test package pattern", findings)
	}
}

// TestRunGoTest_DemotesOutOfScopeMetaPredicateToSkip is the WIRING proof at
// the verdict level: an out-of-scope predicate that would have been RED lands
// as SKIP with a loud note, and the verdict PASSes on the cycle's real
// predicates — the 1123 shape (auditor PASS + one broad red) becomes a
// truthful PASS instead of a false FAIL.
func TestRunGoTest_DemotesOutOfScopeMetaPredicateToSkip(t *testing.T) {
	root := t.TempDir()
	modDir := filepath.Join(root, "go")
	cycleDir := filepath.Join(modDir, "acs", "cycle9999")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module github.com/x/go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "predicates_test.go"), []byte(outOfScopeSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	// Touched set injected via the git seam (production derives it from git,
	// never the agent-written handoff — gate-weakening review finding).
	injectTouched(t, []string{"./internal/bridge/..."})

	raw := goStream(
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_006_suites_stay_green", "run"),
		`{"Action":"output","Package":"github.com/x/go/acs/cycle9999","Test":"TestC9999_006_suites_stay_green","Output":"--- FAIL: contaminated sweep\n"}`,
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_006_suites_stay_green", "fail"),
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_001_scoped_ok", "run"),
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_001_scoped_ok", "pass"),
	)

	v, err := Run(Options{Root: root, ProjectRoot: root, Cycle: 9999, GoExec: seamGo(raw, &fakeExitErr{1})})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.RedCount != 0 {
		t.Fatalf("red_count = %d (%v), want 0 — the out-of-scope meta-predicate's contamination-shaped failure must demote to SKIP, not fail the cycle", v.RedCount, v.RedIDs)
	}
	var demoted *Result
	for i := range v.Results {
		if v.Results[i].ACID == "cycle9999/TestC9999_006_suites_stay_green" {
			demoted = &v.Results[i]
		}
	}
	if demoted == nil || demoted.ResultStr != "skip" {
		t.Fatalf("out-of-scope predicate result = %+v, want skip", demoted)
	}
	if demoted.EvidenceNote == "" {
		t.Error("demotion carries no EvidenceNote — a silently skipped gate is a gate-weakening path; the verdict must say WHY")
	}
	if len(v.Warnings) == 0 {
		t.Error("verdict carries no warning for the demotion — the operator must see out-of-scope authoring without opening results")
	}
	if v.Verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS on the scoped predicate alone", v.Verdict)
	}
}

// injectTouched swaps the git-derived touched-set seam for the test's literal
// set, restoring it on cleanup (repo idiom: seam var + t.Cleanup).
func injectTouched(t *testing.T, touched []string) {
	t.Helper()
	orig := scopeLintChangedPackages
	scopeLintChangedPackages = func(string) []string { return touched }
	t.Cleanup(func() { scopeLintChangedPackages = orig })
}

// TestRunGoTest_DemotionFloorCancelsWhenAllOwnPredicatesWouldSkip pins the
// gate-weakening guard from adversarial review: if every live own predicate is
// out-of-scope, demotion is CANCELLED — a stray broad literal in each
// predicate must not let a cycle ship with zero live own predicates (a real
// red would vanish with them).
func TestRunGoTest_DemotionFloorCancelsWhenAllOwnPredicatesWouldSkip(t *testing.T) {
	root := t.TempDir()
	modDir := filepath.Join(root, "go")
	cycleDir := filepath.Join(modDir, "acs", "cycle9999")
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// ONE predicate, out of scope, genuinely red.
	src := `package cycle9999

import "testing"

func TestC9999_006_suites_stay_green(t *testing.T) {
	_ = []string{"go", "test", "./internal/core/..."}
}
`
	if err := os.WriteFile(filepath.Join(cycleDir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	injectTouched(t, []string{"./internal/bridge/..."})

	raw := goStream(
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_006_suites_stay_green", "run"),
		goLine("github.com/x/go/acs/cycle9999", "TestC9999_006_suites_stay_green", "fail"),
	)
	v, err := Run(Options{Root: root, ProjectRoot: root, Cycle: 9999, GoExec: seamGo(raw, &fakeExitErr{1})})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.RedCount != 1 {
		t.Fatalf("red_count = %d, want 1 — demoting the ONLY live own predicate would leave the gate empty; the floor must cancel and keep the red", v.RedCount)
	}
	found := false
	for _, w := range v.Warnings {
		if strings.Contains(w, "CANCELLED") {
			found = true
		}
	}
	if !found {
		t.Errorf("floor cancellation must be loud in Warnings; got %v", v.Warnings)
	}
}
