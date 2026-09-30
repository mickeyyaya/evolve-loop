//go:build acs

package cycle348

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
	skillcheckPkg = "./internal/skillcheck/..."
	rollbackPkg   = "./internal/rollback/..."
)

func TestC348_001_SkillcheckCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, skillcheckPkg)
	if pct < 90.0 {
		t.Errorf("RED: internal/skillcheck coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add tests exercising:\n"+
			"  nameMismatches: (a) missing skills dir → 'read skills dir' error,\n"+
			"    (b) invalid frontmatter YAML → 'unparseable' message,\n"+
			"    (c) name != dir → DRIFT message\n"+
			"  parallelSubtaskCount: (a) nil/zero raw → 0, (b) invalid JSON → 0,\n"+
			"    (c) array value → len(array)\n"+
			"  inspect: non-skill dir (no SKILL.md) edge case",
			pct)
	}
}

func TestC348_002_SkillcheckNameMismatchesCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, skillcheckPkg)
	pct := funcCoverage(funcOut, "nameMismatches")
	switch {
	case pct < 0:
		t.Errorf("RED: nameMismatches not found in `go tool cover -func` output\n"+
			"Expected in go/internal/skillcheck/skillcheck.go:207\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct < 90.0:
		t.Errorf("RED: nameMismatches coverage = %.1f%%, want >= 90.0%%\n"+
			"Builder must add tests for: missing-skills-dir, invalid-frontmatter,\n"+
			"and name!=dir branches. All three branches require a temp-dir fixture.\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 30))
	}
}

func TestC348_003_RollbackCoverageFloor(t *testing.T) {
	_, pct := coverFuncOutput(t, rollbackPkg)
	if pct < 92.0 {
		t.Errorf("RED: internal/rollback coverage = %.1f%%, want >= 92.0%%\n"+
			"deleteRemoteTagWith is already at 100%% — the gaps are elsewhere:\n"+
			"  revertAndShipWith: add tests where the fake binary exits 1 (→ 'local-only')\n"+
			"    and exits 0 (→ 'reverted') via EVOLVE_GO_BIN pointing to a temp script\n"+
			"  defaultDeleteRemoteTag (0%%): smoke-call with a temp non-git dir (→ 'not-present')\n"+
			"  defaultRevertAndShip (0%%): smoke-call with a temp non-git dir (→ 'failed')\n"+
			"  appendLedger (76.9%%): exercise the write-error path (parent is a file)",
			pct)
	}
}

func TestC348_004_RollbackRevertAndShipWithCovered(t *testing.T) {
	funcOut, _ := coverFuncOutput(t, rollbackPkg)
	pct := funcCoverage(funcOut, "revertAndShipWith")
	switch {
	case pct < 0:
		t.Errorf("RED: revertAndShipWith not found in `go tool cover -func` output\n"+
			"Expected in go/internal/rollback/rollback.go:361\n"+
			"Coverage output:\n%s", tail(funcOut, 30))
	case pct < 80.0:
		t.Errorf("RED: revertAndShipWith coverage = %.1f%%, want >= 80.0%%\n"+
			"Builder must add tests exercising exec.Command(binPath, \"ship\", ...):\n"+
			"  (a) t.Setenv(\"EVOLVE_GO_BIN\", fakeBin) where fakeBin exits 1 → 'local-only'\n"+
			"  (b) t.Setenv(\"EVOLVE_GO_BIN\", fakeBin) where fakeBin exits 0 → 'reverted'\n"+
			"fakeBin = a temp script with #!/bin/sh + exit 0/1 written to t.TempDir()\n"+
			"Coverage output:\n%s", pct, tail(funcOut, 30))
	}
}
