//go:build acs

package cycle1846

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureHeader = "package fixture\n\nimport (\n\t\"errors\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\n"

const copiedExitErrorCode = fixtureHeader + `func TestFixture_CopiedExitErrorCode(t *testing.T) {
	err := exec.Command("go", "run", "./cmd/evolve", "eval").Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatal(err)
	}
	code := ee.ExitCode()
	if code != 3 {
		t.Fatalf("want 3, got %d", code)
	}
}
`

const subprocessOutputGoRunCode = fixtureHeader + `func TestFixture_SubprocessOutputGoRunCode(t *testing.T) {
	_, _, code, _ := acsassert.SubprocessOutput("go", "run", "./cmd/evolve", "eval")
	if code != 3 {
		t.Fatalf("want 3, got %d", code)
	}
}
`

const reachableAndBuiltBinaryCodes = fixtureHeader + `func TestFixture_GoRunCodeOne(t *testing.T) {
	_, _, code, _ := acsassert.SubprocessOutput("go", "run", ".")
	if code != 1 {
		t.Fatal("want 1")
	}
}

func TestFixture_GoRunCodeZero(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	_ = cmd.Run()
	code := cmd.ProcessState.ExitCode()
	if code != 0 {
		t.Fatal("want 0")
	}
}

func TestFixture_GoRunWithFlagsCodeTwo(t *testing.T) {
	_, _, code, _ := acsassert.SubprocessOutput("go", "run", "-race", ".")
	if code != 2 {
		t.Fatal("want 2")
	}
}

func TestFixture_BuiltBinaryCodeThree(t *testing.T) {
	_, _, code, _ := acsassert.SubprocessOutput(t.TempDir()+"/evolve", "eval")
	if code != 3 {
		t.Fatal("want 3")
	}
}

func TestFixture_BuiltBinaryCopiedCodeTwo(t *testing.T) {
	cmd := exec.Command(t.TempDir()+"/evolve", "eval")
	_ = cmd.Run()
	code := cmd.ProcessState.ExitCode()
	if code != 2 {
		t.Fatal("want 2")
	}
}
`

const minimalEval = "---\nscore_cap:\n  - criterion: \"fixture criterion\"\n    max_if_missing: 5\n    evidence: \"cd go && go test -count=1 -run '^TestLintUnsatisfiablePredicates_GoRunExitCodeRuleFollowsExitCodeVariables$' ./internal/evalqualitycheck/\"\n---\n\n# Eval: fixture\n"

func qualityCheckStdout(t *testing.T, predicateSrc string) string {
	t.Helper()
	dir := t.TempDir()
	predicateDir := filepath.Join(dir, "cycle9999")
	if err := os.MkdirAll(predicateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(predicateDir, "predicates_test.go"), []byte(predicateSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	evalPath := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(evalPath, []byte(minimalEval), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	guardcmd.RunEval([]string{"quality-check", evalPath, "-predicates", predicateDir}, nil, &stdout, &stderr)
	out := stdout.String()
	if !strings.Contains(out, "unsatisfiable-lint: linted 1 file(s)") {
		t.Fatalf("the unsatisfiable lint must report a receipt for the one fixture file:\nstdout:\n%s\nstderr:\n%s", out, stderr.String())
	}
	return out
}

func TestC1846_001_QualityCheckFlagsGoRunExitCodeCopiedIntoAVariable(t *testing.T) {
	out := qualityCheckStdout(t, copiedExitErrorCode)
	if !strings.Contains(out, "unsatisfiable[go-run-exit-code] predicates_test.go:TestFixture_CopiedExitErrorCode") {
		t.Errorf("code := ee.ExitCode(); code != 3 after go run must be flagged go-run-exit-code:\n%s", out)
	}
	if !strings.Contains(out, "verdict: WARN") {
		t.Errorf("an unsatisfiable finding must raise the verdict to WARN:\n%s", out)
	}
}

func TestC1846_002_QualityCheckFlagsSubprocessOutputGoRunExitCode(t *testing.T) {
	out := qualityCheckStdout(t, subprocessOutputGoRunCode)
	if !strings.Contains(out, "unsatisfiable[go-run-exit-code] predicates_test.go:TestFixture_SubprocessOutputGoRunCode") {
		t.Errorf("SubprocessOutput(\"go\", \"run\", ...) code != 3 must be flagged go-run-exit-code:\n%s", out)
	}
	if !strings.Contains(out, "exit code 3") {
		t.Errorf("the finding must name the unreachable code 3:\n%s", out)
	}
}

func TestC1846_003_QualityCheckSparesReachableCodesAndBuiltBinaries(t *testing.T) {
	out := qualityCheckStdout(t, reachableAndBuiltBinaryCodes)
	if strings.Contains(out, "unsatisfiable[") {
		t.Errorf("reachable go run codes (0, 1, 2 with go flags) and built-binary codes must stay unflagged:\n%s", out)
	}
	if !strings.Contains(out, "unsatisfiable-lint: linted 1 file(s)") || !strings.Contains(out, "0 advisory finding(s)") {
		t.Errorf("the lint must run and report zero findings on the reachable fixture:\n%s", out)
	}
}

func runNamedTest(t *testing.T, name string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/evalqualitycheck")
	if code != 0 || err != nil {
		t.Errorf("go test -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
	}
}

func TestC1846_004_UnitTableFollowsExitCodeVariables(t *testing.T) {
	runNamedTest(t, "TestLintUnsatisfiablePredicates_GoRunExitCodeRuleFollowsExitCodeVariables")
}

func TestC1846_005_GoAcsCorpusStillHasNoProofKindFinding(t *testing.T) {
	runNamedTest(t, "TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding")
}
