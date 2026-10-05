//go:build acs

package cycle1788

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const invertedIdiomSource = `package fixture

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestInvertedIdiom(t *testing.T) {
	if acsassert.FileContains(t, "f.go", "retiredFlag") {
		t.Errorf("retiredFlag still present")
	}
}
`

const negatedPositiveNamesAbsenceSource = `package fixture

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestPositiveWithAbsenceMessage(t *testing.T) {
	if !acsassert.FileContains(t, "f.go", "worktreeTree == headTree") {
		t.Errorf("f.go still holds the duplicated inline comparison")
	}
}
`

const regexInvertedSource = `package fixture

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestRegexInverted(t *testing.T) {
	if acsassert.FileMatchesRegex(t, "f.go", "old[A-Z]+") {
		t.Fatalf("old marker remains")
	}
}
`

const satisfiableSource = `package fixture

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestAbsenceWithNegativePrimitive(t *testing.T) {
	if !acsassert.FileNotContains(t, "f.go", "retiredFlag") {
		t.Errorf("f.go still holds retiredFlag")
	}
}

func TestPositiveWithMissingMessage(t *testing.T) {
	if !acsassert.FileContains(t, "f.go", "newFlag") {
		t.Errorf("newFlag missing from f.go")
	}
}

func TestBareConditionBranchDoesNotReportFailure(t *testing.T) {
	if acsassert.FileContains(t, "f.go", "optional") {
		t.Logf("optional section present")
	}
}
`

func writeFixtureDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func lintFixture(t *testing.T, src string) evalqualitycheck.UnsatisfiableLintReport {
	t.Helper()
	report, err := evalqualitycheck.LintUnsatisfiablePredicates(writeFixtureDir(t, src))
	if err != nil {
		t.Fatalf("LintUnsatisfiablePredicates: %v", err)
	}
	if report.Linted() != 1 {
		t.Fatalf("receipt must show the one fixture file parsed; Linted=%d Files=%v", report.Linted(), report.Files)
	}
	return report
}

func flaggedFuncs(report evalqualitycheck.UnsatisfiableLintReport) map[string]bool {
	out := map[string]bool{}
	for _, f := range report.Findings {
		out[f.Func] = true
	}
	return out
}

func TestC1788_001_InvertedFileContainsIdiomIsFlagged(t *testing.T) {
	report := lintFixture(t, invertedIdiomSource)
	if !flaggedFuncs(report)["TestInvertedIdiom"] {
		t.Errorf("RED: FileContains used as a bare failure condition (red on the absent state, the cycle-352/1488 class) not flagged; findings=%+v", report.Findings)
	}
	for _, f := range report.Findings {
		if f.File != "predicates_test.go" || strings.TrimSpace(f.Reason) == "" {
			t.Errorf("RED: finding must name its file and carry a reason: %+v", f)
		}
	}
}

func TestC1788_002_PositivePrimitiveWithAbsenceMessageIsFlagged(t *testing.T) {
	report := lintFixture(t, negatedPositiveNamesAbsenceSource)
	if !flaggedFuncs(report)["TestPositiveWithAbsenceMessage"] {
		t.Errorf("RED: FileContains whose failure text demands absence (TestC1488_003 shape) not flagged; findings=%+v", report.Findings)
	}
}

func TestC1788_003_RegexFamilyInvertedIdiomIsFlagged(t *testing.T) {
	report := lintFixture(t, regexInvertedSource)
	if !flaggedFuncs(report)["TestRegexInverted"] {
		t.Errorf("RED: FileMatchesRegex inverted idiom not flagged (family coverage); findings=%+v", report.Findings)
	}
}

func TestC1788_004_SatisfiableAbsenceAndPresenceShapesAreNotFlagged(t *testing.T) {
	report := lintFixture(t, satisfiableSource)
	if len(report.Findings) != 0 {
		t.Errorf("RED: FileNotContains, positive FileContains with a missing-message, and a non-failing bare branch are all satisfiable; got %+v", report.Findings)
	}
}

func TestC1788_005_LintRejectsMissingPathAndEmptyDirLoudly(t *testing.T) {
	if _, err := evalqualitycheck.LintUnsatisfiablePredicates(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Errorf("RED: a nonexistent path must be an error, not a clean report")
	}
	if _, err := evalqualitycheck.LintUnsatisfiablePredicates(t.TempDir()); err == nil {
		t.Errorf("RED: a directory with no .go files must be an error, not a clean report")
	}
}

func TestC1788_006_CorpusSweepLintsEveryCyclePackage(t *testing.T) {
	acsDir := filepath.Join(acsassert.RepoRoot(t), "go", "acs")
	entries, err := os.ReadDir(acsDir)
	if err != nil {
		t.Fatal(err)
	}
	cyclePkgs, linted, findings := 0, 0, 0
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle") {
			continue
		}
		cyclePkgs++
		report, err := evalqualitycheck.LintUnsatisfiablePredicates(filepath.Join(acsDir, e.Name()))
		if err != nil {
			t.Errorf("RED: corpus sweep failed on %s: %v", e.Name(), err)
			continue
		}
		linted += report.Linted()
		for _, f := range report.Findings {
			findings++
			t.Logf("corpus finding: %s/%s:%s — %s", e.Name(), f.File, f.Func, f.Reason)
			if f.Func == "" || f.File == "" || f.Reason == "" {
				t.Errorf("RED: incomplete finding in %s: %+v", e.Name(), f)
			}
		}
	}
	if cyclePkgs < 50 || linted < cyclePkgs {
		t.Errorf("RED: sweep must cover the whole corpus; cyclePkgs=%d linted=%d", cyclePkgs, linted)
	}
	t.Logf("corpus sweep: %d cycle packages, %d files linted, %d findings", cyclePkgs, linted, findings)
	self, err := evalqualitycheck.LintUnsatisfiablePredicates(filepath.Join(acsDir, "cycle1788"))
	if err != nil || len(self.Findings) != 0 {
		t.Errorf("RED: this cycle's own predicates must be satisfiable: err=%v findings=%+v", err, self.Findings)
	}
}

const passingEval = "# Eval\n\n```bash\ngo test -count=1 -run TestSomething ./internal/foo\n```\n"

func runQualityCheck(t *testing.T, predicatesDir string) (string, int) {
	t.Helper()
	evalPath := filepath.Join(t.TempDir(), "eval.md")
	if err := os.WriteFile(evalPath, []byte(passingEval), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/evolve", "eval", "quality-check", evalPath, "-predicates", predicatesDir)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%v: %v", cmd.Args, err)
	}
	return string(out), code
}

func TestC1788_007_QualityCheckPreflightWarnsOnUnsatisfiablePredicate(t *testing.T) {
	cleanOut, cleanCode := runQualityCheck(t, writeFixtureDir(t, satisfiableSource))
	if cleanCode != 0 {
		t.Fatalf("RED: control (satisfiable predicates) must PASS rc=0, got rc=%d:\n%s", cleanCode, cleanOut)
	}
	out, code := runQualityCheck(t, writeFixtureDir(t, invertedIdiomSource))
	if code != 1 {
		t.Errorf("RED: unsatisfiable predicate must raise PASS→WARN (rc=1) through the real quality-check entry point, got rc=%d:\n%s", code, out)
	}
	if !strings.Contains(out, "unsatisfiable[") || !strings.Contains(out, "TestInvertedIdiom") {
		t.Errorf("RED: pre-flight output must carry an unsatisfiable[...] line naming the function:\n%s", out)
	}
	if !strings.Contains(out, "unsatisfiable-lint: linted 1 file(s)") {
		t.Errorf("RED: pre-flight must print the unconditional linted-N-files receipt:\n%s", out)
	}
}

func TestC1788_008_PreflightNeverLowersATautologyHalt(t *testing.T) {
	evalPath := filepath.Join(t.TempDir(), "eval.md")
	if err := os.WriteFile(evalPath, []byte("```bash\n:\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/evolve", "eval", "quality-check", evalPath, "-predicates", writeFixtureDir(t, invertedIdiomSource))
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Errorf("RED: Level-0 tautology must stay HALT (rc=2) with an advisory finding present; err=%v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "unsatisfiable[") {
		t.Errorf("RED: unsatisfiable lint must still report alongside the HALT:\n%s", out)
	}
}

// acs-predicate: config-check
func TestC1788_009_NewExportsNamedForRepoWideApicoverGate(t *testing.T) {
	named := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "evalqualitycheck", "apicover_named_test.go")
	for _, sym := range []string{"LintUnsatisfiablePredicates", "UnsatisfiableFinding", "UnsatisfiableLintReport"} {
		if !acsassert.FileContains(t, named, sym) {
			t.Errorf("RED: %s does not name %s — ADR-0069 apicover gate will reject the package", named, sym)
		}
	}
}
