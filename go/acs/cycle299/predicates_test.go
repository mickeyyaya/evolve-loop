//go:build acs

package cycle299

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

var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func coverFuncOutput(t *testing.T, pkg string) string {
	t.Helper()
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "c.cover")
	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-coverprofile="+profile, pkg); err != nil || code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d, err=%v) — add the new test file / fix regressions:\n%s",
			pkg, code, err, tail(stderr, 40))
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 20))
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

const (
	flockPkg    = "./internal/adapters/flock/..."
	runleasePkg = "./internal/runlease/..."
	sessionPkg  = "./internal/sessionrecord/..."
)

func TestC299_001_FlockCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, flockPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/adapters/flock coverage %.1f%% < %.1f%% floor — "+
			"add flock_test.go covering Lock + MkdirAll/OpenFile/flockFn error branches", pct, floor)
	}
}

func TestC299_002_FlockErrorBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, flockPkg)
	if pct := funcCoverage(t, out, "Lock"); pct < floor {
		t.Errorf("RED: flock.Lock coverage %.1f%% < %.1f%% — the flockFn LOCK_EX error "+
			"branch (and/or the release closure) is not exercised; inject an error via the "+
			"`var flockFn` seam and assert Lock returns it", pct, floor)
	}
}

func TestC299_003_RunleaseCoverageFloor(t *testing.T) {
	const floor = 95.0
	out := coverFuncOutput(t, runleasePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/runlease coverage %.1f%% < %.1f%% floor — "+
			"cover Write's CreateTemp/Write/Close/Rename error returns", pct, floor)
	}
}

func TestC299_004_RunleaseWriteCoverage(t *testing.T) {
	const floor = 80.0
	out := coverFuncOutput(t, runleasePkg)
	if pct := funcCoverage(t, out, "Write"); pct < floor {
		t.Errorf("RED: runlease.Write coverage %.1f%% < %.1f%% — exercise the tmp/write/"+
			"close/rename error returns (e.g. a non-writable runDir, a rename collision)", pct, floor)
	}
}

func TestC299_005_SessionrecordCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, sessionPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/sessionrecord coverage %.1f%% < %.1f%% floor — "+
			"test RunScopeToken edge cases + Append open/write/close error branches", pct, floor)
	}
}

func TestC299_006_SessionrecordRunScopeTokenCoverage(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, sessionPkg)
	if pct := funcCoverage(t, out, "RunScopeToken"); pct < floor {
		t.Errorf("RED: sessionrecord.RunScopeToken coverage %.1f%% < %.1f%% — test the "+
			">8-char truncation branch and the short-input path", pct, floor)
	}
}
