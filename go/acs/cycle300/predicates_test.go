//go:build acs

package cycle300

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
		t.Fatalf("RED: %s test run failed (exit=%d, err=%v) — add the new tests / fix regressions:\n%s",
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
	gcPkg  = "./internal/gc/..."
	lpfPkg = "./internal/looppreflight/..."
)

func TestC300_001_GCErrorPathBranchesCovered(t *testing.T) {
	out := coverFuncOutput(t, gcPkg)
	if pct := funcCoverage(t, out, "Plan"); pct < 100.0 {
		t.Errorf("RED: gc.Plan coverage %.1f%% < 100%% — cover the EvolveDir-absolute "+
			"guard (relative/empty path rejected) and the nil-Now wall-clock fallback", pct)
	}
	if pct := funcCoverage(t, out, "dirEntriesOlderThan"); pct < 100.0 {
		t.Errorf("RED: gc.dirEntriesOlderThan coverage %.1f%% < 100%% — exercise the "+
			"filter-reject callback (an entry the filter returns false for must be skipped)", pct)
	}
}

func TestC300_002_GCCoverageFloor(t *testing.T) {
	const floor = 98.0
	out := coverFuncOutput(t, gcPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/gc coverage %.1f%% < %.1f%% floor — cover the Plan guards, "+
			"nil-Now, the dirEntriesOlderThan filter-reject, Discover missing-runs, and the "+
			"currentWorkspace read-error branch", pct, floor)
	}
}

func TestC300_003_LoopPreflightSafetyGateBranchesCovered(t *testing.T) {
	out := coverFuncOutput(t, lpfPkg)
	if pct := funcCoverage(t, out, "String"); pct < 100.0 {
		t.Errorf("RED: looppreflight.CheckLevel.String coverage %.1f%% < 100%% — cover the "+
			"out-of-range \"unknown\" branch (cast an int outside LevelPass/Warn/Halt)", pct)
	}
	if pct := funcCoverage(t, out, "checkPipelineStructure"); pct < 100.0 {
		t.Errorf("RED: looppreflight.checkPipelineStructure coverage %.1f%% < 100%% — drive "+
			"all three Halt-accumulation arms (false factoryLookup, erroring profileLister, "+
			"erroring profileGetter)", pct)
	}
}

func TestC300_004_LoopPreflightCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, lpfPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/looppreflight coverage %.1f%% < %.1f%% floor — cover String "+
			"unknown, defaultDiskFreeBytes Statfs-error, checkPipelineStructure gaps, resolve "+
			"default seams, and sandboxWanted no-sandbox-profiles", pct, floor)
	}
}
