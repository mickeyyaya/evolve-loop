//go:build acs

package cycle347

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func coverFuncOutput(t *testing.T, pkg string) (funcOut string, totalPct float64) {
	t.Helper()
	dir := goDir(t)
	tmp := t.TempDir()
	cp := filepath.Join(tmp, "c.out")
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"-coverprofile="+cp,
		"-coverpkg="+pkg,
		pkg,
	)
	if code != 0 || err != nil {
		t.Fatalf("go test %s failed (exit=%d): %v\nOutput:\n%s", pkg, code, err, tail(out, 40))
	}
	funcOut2, _, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+cp)
	totalRe := regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)
	m := totalRe.FindStringSubmatch(funcOut2)
	if m == nil {
		t.Fatalf("could not parse total from `go tool cover -func`:\n%s", funcOut2)
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	return funcOut2, pct
}

func funcCoverage(funcOut, funcName string) float64 {
	re := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(funcName) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(funcOut)
	if m == nil {
		return -1.0
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	return pct
}

const (
	skillcheckPkg  = "./internal/skillcheck/..."
	codequalityPkg = "./internal/codequality/..."
	auditPkg       = "./internal/phases/audit/..."
)

func TestC347_001_SkillcheckRunCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, skillcheckPkg)
	if pct < 70.0 {
		t.Errorf("RED: internal/skillcheck coverage = %.1f%%, want >= 70.0%%\n"+
			"Builder must add TestRun_* tests exercising Run(projectRoot, write, stdout, stderr)\n"+
			"across: write=false no-drift, write=false drift (exit 2), write=true drift, invalid root (exit 1)",
			pct)
	}
}

func TestC347_002_SkillcheckRunFunctionNonZero(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, skillcheckPkg)
	pct := funcCoverage(funcOut, "Run")
	switch {
	case pct < 0:
		t.Errorf("RED: Run function not found in `go tool cover -func` output\n"+
			"Expected function at go/internal/skillcheck/skillcheck.go:135\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct == 0.0:
		t.Errorf("RED: Run function coverage = 0.0%% — no test calls Run()\n"+
			"Builder must add tests that call skillcheck.Run(projectRoot, write, stdout, stderr)\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	}
}

func TestC347_003_CodequalityCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, codequalityPkg)
	if pct < 90.0 {
		t.Errorf("RED: internal/codequality coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add:\n"+
			"  TestFirstLine_NoNewline — firstLine(s) when s contains no newline\n"+
			"  TestUnformattedGoFiles_GofmtMissing — t.Setenv(\"PATH\", \"\") to hide gofmt binary",
			pct)
	}
}

func TestC347_004_FirstLineFunctionFullyCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, codequalityPkg)
	pct := funcCoverage(funcOut, "firstLine")
	switch {
	case pct < 0:
		t.Errorf("RED: firstLine function not found in `go tool cover -func` output\n"+
			"Expected at go/internal/codequality/gofmt.go:61\n"+
			"Coverage output:\n%s", tail(funcOut, 20))
	case pct < 100.0:
		t.Errorf("RED: firstLine coverage = %.1f%%, want 100%%\n"+
			"Builder must add TestFirstLine_NoNewline exercising the return-s branch\n"+
			"(line 64: when strings.IndexByte finds no newline, return s unchanged)\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 20))
	}
}

func TestC347_005_NewDefaultWiresSkillsDriftCheckTestPasses(t *testing.T) {
	dir := goDir(t)
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestNewDefault_WiresSkillsDriftCheck",
		"./internal/phases/audit/...",
	)
	if err != nil && code != 0 {
		t.Fatalf("go test subprocess error (exit=%d): %v\nOutput:\n%s", code, err, tail(out, 30))
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestNewDefault_WiresSkillsDriftCheck`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestNewDefault_WiresSkillsDriftCheck not found as PASS (exit=%d)\n"+
			"Builder must add TestNewDefault_WiresSkillsDriftCheck to\n"+
			"go/internal/phases/audit/audit_skillsdrift_test.go\n"+
			"It should mirror TestNewDefault_WiresGofmtCheck: create a worktree with a\n"+
			"drifted SKILL.md (mutated generated region), pre-stage acs-verdict.json red=0,\n"+
			"run NewDefault audit, assert Verdict=FAIL with a skills-drift diagnostic.\n"+
			"Use testing.Short() guard (same as the gofmt analog).\n"+
			"Output:\n%s",
			code, tail(out, 30))
	}
}

func TestC347_006_AuditPackageCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, auditPkg)
	if pct < 91.0 {
		t.Errorf("RED: internal/phases/audit coverage = %.1f%%, want >= 91.0%%\n"+
			"Builder must add:\n"+
			"  TestNewDefault_WiresSkillsDriftCheck — end-to-end skills-drift gate verification\n"+
			"  Tests for gofmtCheckDefault/skillsDriftCheckDefault Worktree=\"\" fallback paths\n"+
			"  (these are at 66.7%% each today; the fallback to ProjectRoot is uncovered)",
			pct)
	}
}
