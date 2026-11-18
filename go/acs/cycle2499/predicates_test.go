//go:build acs

package cycle2499

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

const predicateHeader = `//go:build acs

package cycle4242

import (
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	_ = exec.Command
	_ = evalqualitycheck.Check
	_ = acsassert.RepoRoot
)

`

const fileContainsOnly = predicateHeader + `
func TestC4242_ThingIsDeclared(t *testing.T) {
	if !acsassert.FileContains(t, "go/internal/thing/thing.go", "func NewThing(") {
		t.Errorf("NewThing missing")
	}
}
`

const regexOnly = predicateHeader + `
func TestC4242_ThingHasCaller(t *testing.T) {
	if !acsassert.FileMatchesRegex(t, "go/internal/thing/thing.go", "NewThing\\(.*\\)") {
		t.Errorf("NewThing call missing")
	}
}
`

const fileContainsAndRegexOnly = predicateHeader + `
func TestC4242_ThingShapeInSource(t *testing.T) {
	ok := acsassert.FileContains(t, "go/internal/thing/thing.go", "func NewThing(")
	if !ok || !acsassert.FileMatchesRegex(t, "go/internal/thing/thing.go", "Thing") {
		t.Errorf("thing shape missing")
	}
}
`

var runsTheCode = map[string]string{
	"subprocess": predicateHeader + `
func TestC4242_ToolRejectsBadInput(t *testing.T) {
	_, _, code, err := acsassert.SubprocessOutput("tool", "--bogus")
	if err != nil || code == 0 {
		t.Errorf("tool accepted bad input: code=%d err=%v", code, err)
	}
}
`,
	"exec-command": predicateHeader + `
func TestC4242_ToolRuns(t *testing.T) {
	if err := exec.Command("tool", "--version").Run(); err != nil {
		t.Errorf("tool does not run: %v", err)
	}
}
`,
	"direct-call": predicateHeader + `
func TestC4242_CheckHaltsOnTautology(t *testing.T) {
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: "eval.md"})
	if err != nil || res.Overall != evalqualitycheck.LevelHalt {
		t.Errorf("tautology not halted: %v %v", res.Overall, err)
	}
}
`,
	"mixed-with-source-grep": predicateHeader + `
func TestC4242_ToolRunsAndIsDeclared(t *testing.T) {
	if !acsassert.FileContains(t, "go/internal/thing/thing.go", "func NewThing(") {
		t.Errorf("NewThing missing")
	}
	_, _, code, _ := acsassert.SubprocessOutput("tool", "run")
	if code != 0 {
		t.Errorf("tool failed: %d", code)
	}
}
`,
}

const oneBadOneGood = predicateHeader + `
func TestC4242_BadSourceGrepOnly(t *testing.T) {
	if !acsassert.FileContains(t, "go/internal/thing/thing.go", "func NewThing(") {
		t.Errorf("NewThing missing")
	}
}

func TestC4242_GoodRunsTool(t *testing.T) {
	_, _, code, _ := acsassert.SubprocessOutput("tool", "run")
	if code != 0 {
		t.Errorf("tool failed: %d", code)
	}
}

func helperNotAPredicate(t *testing.T) bool {
	return acsassert.FileContains(t, "go/internal/thing/thing.go", "x")
}
`

const waivedConfigCheck = predicateHeader + `
// acs-predicate: config-check
func TestC4242_PolicyKeyPresent(t *testing.T) {
	if !acsassert.FileContains(t, ".evolve/policy.json", "acs") {
		t.Errorf("policy key missing")
	}
}
`

func lintSource(t *testing.T, src string) evalqualitycheck.SourceTextLintReport {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := evalqualitycheck.LintSourceTextPredicates(dir)
	if err != nil {
		t.Fatalf("LintSourceTextPredicates: %v", err)
	}
	if report.Linted() != 1 {
		t.Fatalf("linted %d file(s), want 1 — a lint that inspects nothing is not a clean result", report.Linted())
	}
	return report
}

func reviewAtTDD(t *testing.T, stage config.Stage, src string) core.ReviewResult {
	t.Helper()
	worktree := t.TempDir()
	dir := filepath.Join(worktree, "go", "acs", "cycle4242")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "cycle-4242")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	in := core.ReviewInput{Phase: "tdd", Workspace: workspace, Worktree: worktree, ProjectRoot: t.TempDir()}
	return evalgate.NewReviewer(stage).Review(context.Background(), in)
}

func TestC2499_001_FileContainsOnlyPredicateIsRefused(t *testing.T) {
	report := lintSource(t, fileContainsOnly)
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", report.Findings)
	}
	f := report.Findings[0]
	if f.Func != "TestC4242_ThingIsDeclared" || f.File != "predicates_test.go" || f.Reason == "" {
		t.Errorf("finding = %+v, want the FileContains-only predicate named with its file and a reason", f)
	}
}

func TestC2499_002_FileMatchesRegexOnlyPredicateIsRefused(t *testing.T) {
	report := lintSource(t, regexOnly)
	if len(report.Findings) != 1 || report.Findings[0].Func != "TestC4242_ThingHasCaller" {
		t.Errorf("findings = %+v, want the FileMatchesRegex-only predicate", report.Findings)
	}
	combined := lintSource(t, fileContainsAndRegexOnly)
	if len(combined.Findings) != 1 || combined.Findings[0].Func != "TestC4242_ThingShapeInSource" {
		t.Errorf("findings = %+v, want the predicate that mixes FileContains with FileMatchesRegex and runs nothing", combined.Findings)
	}
}

func TestC2499_003_PredicateThatRunsTheCodePasses(t *testing.T) {
	for shape, src := range runsTheCode {
		report := lintSource(t, src)
		if len(report.Findings) != 0 {
			t.Errorf("%s: findings = %+v, want none — the predicate exercises the code", shape, report.Findings)
		}
	}
}

func TestC2499_004_OnlyTheSourceTextPredicateOfAMixedFileIsNamed(t *testing.T) {
	report := lintSource(t, oneBadOneGood)
	if len(report.Findings) != 1 || report.Findings[0].Func != "TestC4242_BadSourceGrepOnly" {
		t.Errorf("findings = %+v, want only TestC4242_BadSourceGrepOnly (the runner and the non-Test helper are clean)", report.Findings)
	}
}

func TestC2499_005_ConfigCheckWaiverIsHonoured(t *testing.T) {
	report := lintSource(t, waivedConfigCheck)
	if len(report.Findings) != 0 {
		t.Errorf("findings = %+v, want none — an acs-predicate: config-check waiver marks a config-presence check", report.Findings)
	}
}

func TestC2499_006_ReviewerRefusesSourceTextPredicateAtEnforce(t *testing.T) {
	res := reviewAtTDD(t, config.StageEnforce, fileContainsOnly)
	if res.Approve {
		t.Fatalf("reviewer approved a FileContains-only predicate at enforce; the gate is not wired into the production reviewer")
	}
	for _, want := range []string{"TestC4242_ThingIsDeclared", "source-text"} {
		if !strings.Contains(res.Reason, want) {
			t.Errorf("rejection reason %q must contain %q", res.Reason, want)
		}
	}
}

func TestC2499_007_ReviewerApprovesPredicateThatRunsTheCodeAtEnforce(t *testing.T) {
	for shape, src := range runsTheCode {
		if res := reviewAtTDD(t, config.StageEnforce, src); !res.Approve {
			t.Errorf("%s: reviewer refused a predicate that runs the code: %s", shape, res.Reason)
		}
	}
}

func TestC2499_008_ReviewerOnlyObservesAtShadow(t *testing.T) {
	if res := reviewAtTDD(t, config.StageShadow, fileContainsOnly); !res.Approve {
		t.Errorf("shadow stage blocked a source-text predicate: %s", res.Reason)
	}
}
