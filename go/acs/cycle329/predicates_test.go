//go:build acs

package cycle329

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

const (
	verdictcachePkg = "./internal/verdictcache/..."
	triagecapPkg    = "./internal/triagecap/..."
)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func runGoTest(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(stderr, 30))
	}
	return stdout + stderr, code
}

var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func coverFuncOutput(t *testing.T, pkg string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, pkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the committed coverage tests / fix regressions:\n%s",
			pkg, code, tail(combined, 40))
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

func TestC329_001_VerdictcacheCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, verdictcachePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/verdictcache coverage %.1f%% < %.1f%% floor — exercise (*Store).write's "+
			"mkdir / write-temp error exits, (*Store).Load's non-IsNotExist read error, and NewStore's "+
			"nil-now default. Baseline 79.5%%.", pct, floor)
	}
}

func TestC329_002_VerdictcacheSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", verdictcachePkg)
	if code != 0 {
		t.Errorf("RED: internal/verdictcache suite fails under -race (exit=%d) — the new error-path "+
			"tests must not break the round-trip / persist / degrade-on-corrupt contract:\n%s",
			code, tail(combined, 30))
	}
}

func TestC329_003_StoreWriteErrorExitsExercised(t *testing.T) {
	const floor = 80.0
	out := coverFuncOutput(t, verdictcachePkg)
	if pct := funcCoverage(t, out, "write"); pct < floor {
		t.Errorf("RED: (*Store).write coverage %.1f%% < %.1f%% — add tests that fail os.MkdirAll "+
			"(file-as-dir) and os.WriteFile (read-only .evolve dir). Baseline 63.6%%; marshal/rename "+
			"error arms are out of scope so ~82%% is the practical ceiling.", pct, floor)
	}
}

func TestC329_004_TriagecapCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, triagecapPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/triagecap coverage %.1f%% < %.1f%% floor — add direct tests for "+
			"NewReviewer, readFailedApproaches, CommittedFloorPackages, readWindow. Baseline 86.8%%.", pct, floor)
	}
}

func TestC329_005_TriagecapSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", triagecapPkg)
	if code != 0 {
		t.Errorf("RED: internal/triagecap suite fails under -race (exit=%d) — the new "+
			"uncovered_fns_test.go must not break the floors / demotion / window contract:\n%s",
			code, tail(combined, 30))
	}
}

func TestC329_006_TriagecapZeroCovFunctionsExercised(t *testing.T) {
	const floor = 50.0
	out := coverFuncOutput(t, triagecapPkg)
	for _, fn := range []string{"NewReviewer", "readFailedApproaches", "CommittedFloorPackages", "readWindow"} {
		if pct := funcCoverage(t, out, fn); pct < floor {
			t.Errorf("RED: triagecap.%s coverage %.1f%% < %.1f%% — this function was 0.0%% at baseline "+
				"and must be directly exercised by uncovered_fns_test.go.", fn, pct, floor)
		}
	}
}
