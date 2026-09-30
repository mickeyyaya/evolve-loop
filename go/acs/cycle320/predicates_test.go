//go:build acs

package cycle320

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

const phasecoherencePkg = "./internal/phasecoherence/..."

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

func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, phasecoherencePkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the canonicalRole / dispatchNone "+
			"branch tests / fix regressions:\n%s", phasecoherencePkg, code, tail(combined, 40))
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

func TestC320_001_PhasecoherenceCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/phasecoherence coverage %.1f%% < %.1f%% floor — exercise the dark "+
			"canonicalRole exact-match arms and dispatchNone branches. Baseline 85.2%%.", pct, floor)
	}
}

func TestC320_002_CanonicalRoleBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "canonicalRole"); pct < floor {
		t.Errorf("RED: canonicalRole coverage %.1f%% < 100%% — add table rows for the exact-match "+
			"inputs (scout, builder, build, auditor, audit, intent, memo) so every switch arm runs. "+
			"Baseline 42.9%% (only the default arm).", pct)
	}
}

func TestC320_003_DispatchNoneBranchesCovered(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "dispatchNone"); pct < floor {
		t.Errorf("RED: dispatchNone coverage %.1f%% < 100%% — add a test that returns true for a "+
			"`dispatch: none` persona AND drives the reject arms (unreadable persona → false, "+
			"unparsable/absent frontmatter → false, non-\"none\" dispatch value → false). "+
			"Baseline 75.0%%.", pct)
	}
}

func TestC320_004_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestCoherence|TestCanonicalRole", "-count=1", phasecoherencePkg)
	if code != 0 {
		t.Errorf("RED: the existing phasecoherence contract suite fails (exit=%d) — the new "+
			"coverage tests must not break the coherence / canonicalRole contract:\n%s",
			code, tail(combined, 30))
	}
}
