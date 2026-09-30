//go:build acs

package cycle318

import (
	"errors"
	"os"
	"os/exec"
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

const (
	cmdEvolvePkg = "./cmd/evolve/..."
	ledgerPkg    = "./internal/adapters/ledger/..."
)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func runGoTest(t *testing.T, extraEnv []string, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	cmd := exec.Command("go", full...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err == nil {
		return buf.String(), 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return buf.String(), ee.ExitCode()
	}
	t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(buf.String(), 30))
	return "", -1
}

var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, nil, "-count=1", "-coverprofile="+profile, ledgerPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the new seal I/O tests / fix regressions:\n%s",
			ledgerPkg, code, tail(combined, 40))
	}
	stdout, stderr, code2, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code2 != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code2, err, tail(stderr, 20))
	}
	return stdout
}

func totalCoverage(t *testing.T, out string) float64 {
	t.Helper()
	m := coverTotalRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not parse total coverage from:\n%s", tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable total coverage %q: %v", m[1], err)
	}
	return pct
}

func funcCoverage(t *testing.T, out, fn string) float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\S+:\d+:\s+` + regexp.QuoteMeta(fn) + `\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not find %s() in coverage output (renamed/removed?):\n%s", fn, tail(out, 20))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable %s coverage %q: %v", fn, m[1], err)
	}
	return pct
}

func TestC318_001_FailVerdictBreakerIsolatesFromAmbientEnv(t *testing.T) {
	combined, code := runGoTest(t, []string{"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3"},
		"-run", "TestRunLoop_FailVerdictBreaks", "-count=1", "-short", cmdEvolvePkg)
	if code != 0 {
		t.Errorf("RED: TestRunLoop_FailVerdictBreaks fails when EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 "+
			"is present in the environment (exit=%d) — isolate it (e.g. "+
			"t.Setenv(\"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS\", \"1\")) so the breaker trips on the "+
			"first FAIL and the loop exits rc=2:\n%s", code, tail(combined, 25))
	}
}

func TestC318_002_CmdEvolveSuiteGreenUnderAmbientVar(t *testing.T) {
	combined, code := runGoTest(t, []string{"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3"},
		"-count=1", "-short", cmdEvolvePkg)
	if code != 0 {
		t.Errorf("RED: full cmd/evolve suite fails when EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 is set "+
			"(exit=%d) — the fail-breaker isolation fix must make it green:\n%s", code, tail(combined, 30))
	}
}

func TestC318_003_LedgerCoverageFloor(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/adapters/ledger coverage %.1f%% < %.1f%% floor — add seal I/O tests "+
			"(writeSegment/rewriteLive/readSegment/linesEqual). Baseline 82.4%%.", pct, floor)
	}
}

func TestC318_004_SealContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, nil, "-run", "TestSeal", "-count=1", "-short", ledgerPkg)
	if code != 0 {
		t.Errorf("RED: the TestSeal* contract suite fails (exit=%d) — seal I/O tests must not break "+
			"the chain-preservation / resume-after-crash contract:\n%s", code, tail(combined, 30))
	}
}

func TestC318_005_LinesEqualFalseBranchCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "linesEqual"); pct < floor {
		t.Errorf("RED: linesEqual coverage %.1f%% < 100%% — add a NEGATIVE test driving the "+
			"unequal-elements (and/or length-mismatch) FALSE branch. Baseline 66.7%%.", pct)
	}
}
