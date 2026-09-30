//go:build acs

package cycle332

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalgatePkg = "./internal/evalgate/"
	releasePkg  = "./internal/releasepipeline/"
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

func runGoTest(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(stderr, 30))
	}
	return stdout + stderr, code
}

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

func totalCoverage(t *testing.T, out string) float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^total:\s+\(statements\)\s+([0-9.]+)%`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("could not find total coverage row:\n%s", tail(out, 10))
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unparsable total coverage %q: %v", m[1], err)
	}
	return pct
}

func requirePassLines(t *testing.T, pkg, runRegex string, names []string) {
	t.Helper()
	combined, code := runGoTest(t, "-v", "-count=1", "-run", runRegex, pkg)
	if code != 0 {
		t.Errorf("RED: %s suite fails (exit=%d) for -run %q — the committed tests "+
			"must exist and PASS:\n%s", pkg, code, runRegex, tail(combined, 40))
		return
	}
	for _, n := range names {
		passLine := "--- PASS: " + n
		if !strings.Contains(combined, passLine) {
			t.Errorf("RED: %q not present — %s is missing, renamed, or did not PASS "+
				"(a no-op -run match prints \"no tests to run\" and zero PASS lines):\n%s",
				passLine, n, tail(combined, 40))
		}
	}
}

func TestC332_001_CycleNumFromWorkspaceBothReturnZero(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "cycleNumFromWorkspace"); pct < floor {
		t.Errorf("RED: evalgate.cycleNumFromWorkspace coverage %.1f%% < %.1f%% — cover BOTH return-0 arms: "+
			"a non-cycle basename (nil-match) AND an overflowing all-digit basename "+
			"(cycle-99999999999999999999999999 → strconv.Atoi ErrRange). Baseline 71.4%%; the Atoi arm is "+
			"REACHABLE via overflow (scout's 'unreachable' is wrong) and is required to clear the 93%% floor.",
			pct, floor)
	}
}

func TestC332_002_FencedAfterHeadingNoNewlineElse(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "fencedAfterHeading"); pct < floor {
		t.Errorf("RED: evalgate.fencedAfterHeading coverage %.1f%% < %.1f%% — add a test where the opening "+
			"fence line lacks a trailing newline (report ends at \"\\n```\") and assert it returns (\"\", false). "+
			"Baseline 80.0%%.", pct, floor)
	}
}

func TestC332_003_NewReviewerLogfBodyExercised(t *testing.T) {
	const floor = 100.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := funcCoverage(t, out, "NewReviewer"); pct < floor {
		t.Errorf("RED: evalgate.NewReviewer coverage %.1f%% < %.1f%% — drive Review() to a real violation "+
			"(NewReviewer(config.StageShadow), then Review with an input that trips a gate) so the logf "+
			"closure body runs. Baseline 50.0%%.", pct, floor)
	}
}

func TestC332_004_EvalgatePackageCoverageFloor(t *testing.T) {
	const floor = 93.0
	out := coverFuncOutput(t, evalgatePkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/evalgate coverage %.2f%% < %.1f%% — the four dark branch tests "+
			"(cycleNumFromWorkspace x2, fencedAfterHeading, NewReviewer logf) must all land. Baseline 91.06%%; "+
			"all FOUR statements are required (three alone = 92.74%% < 93.0%%).", pct, floor)
	}
}

func TestC332_005_DefaultReleaseVerifyGuardTestsPass(t *testing.T) {
	requirePassLines(t, releasePkg,
		"TestDefaultReleaseVerify_RelativeRepoRoot|TestDefaultReleaseVerify_MissingBinaryOnDisk",
		[]string{
			"TestDefaultReleaseVerify_RelativeRepoRoot",
			"TestDefaultReleaseVerify_MissingBinaryOnDisk",
		})
}

func TestC332_006_DefaultReleaseVerifyCoverageOffZero(t *testing.T) {
	const floor = 10.0
	out := coverFuncOutput(t, releasePkg)
	if pct := funcCoverage(t, out, "defaultReleaseVerify"); pct < floor {
		t.Errorf("RED: releasepipeline.defaultReleaseVerify coverage %.1f%% < %.1f%% — the relative-repoRoot "+
			"and missing-binary-on-disk guard tests must exercise the early-exit branches. Baseline 0.0%%; "+
			"the cat-file/SHA/re-pin/version/tag arms (need a real git repo) are scout-deferred.", pct, floor)
	}
}

func TestC332_007_DefaultShipBinaryNotFoundDirectTestPass(t *testing.T) {
	requirePassLines(t, releasePkg,
		"TestDefaultShip_BinaryNotFound",
		[]string{"TestDefaultShip_BinaryNotFound"})
}
