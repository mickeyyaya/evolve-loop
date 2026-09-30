//go:build acs

package cycle331

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const ledgerPkg = "./internal/adapters/ledger/"

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

func requirePassLines(t *testing.T, runRegex string, names []string) {
	t.Helper()
	combined, code := runGoTest(t, "-v", "-count=1", "-run", runRegex, ledgerPkg)
	if code != 0 {
		t.Errorf("RED: ledger suite fails (exit=%d) for -run %q — the committed write-error tests "+
			"must exist and PASS:\n%s", code, runRegex, tail(combined, 40))
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

func TestC331_001_SealWriteErrorTestsPass(t *testing.T) {
	requirePassLines(t,
		"TestWriteSegment_MkdirError|TestWriteSegment_CreateTempError|TestRewriteLive_CreateTempError",
		[]string{
			"TestWriteSegment_MkdirError",
			"TestWriteSegment_CreateTempError",
			"TestRewriteLive_CreateTempError",
		})
}

func TestC331_002_WriteSegmentCreateTempCovered(t *testing.T) {
	const floor = 66.0
	out := coverFuncOutput(t, ledgerPkg)
	if pct := funcCoverage(t, out, "writeSegment"); pct < floor {
		t.Errorf("RED: ledger.writeSegment coverage %.1f%% < %.1f%% — add a test driving os.CreateTemp "+
			"to error (read-only parent dir). Baseline 62.5%%; the gzip/sync/close arms are unreachable "+
			"by fs tricks so ~66.7%% is the ceiling.", pct, floor)
	}
}

func TestC331_003_AnchorWriteErrorTestsPass(t *testing.T) {
	requirePassLines(t,
		"TestAnchor_CreateTempError|TestAnchor_RenameError|TestAnchor_GatherError|TestLoadAnchorSHA_CorruptJSON",
		[]string{
			"TestAnchor_CreateTempError",
			"TestAnchor_RenameError",
			"TestAnchor_GatherError",
			"TestLoadAnchorSHA_CorruptJSON",
		})
}

func TestC331_004_AnchorWriteErrorBranchesCovered(t *testing.T) {
	const floor = 75.0
	out := coverFuncOutput(t, ledgerPkg)
	if pct := funcCoverage(t, out, "Anchor"); pct < floor {
		t.Errorf("RED: ledger.Anchor coverage %.1f%% < %.1f%% — add tests for gatherAllLines "+
			"propagation, os.CreateTemp failure (read-only dir), and final os.Rename failure. "+
			"Baseline 65.6%%; marshal + post-open write/close arms are unreachable so ~81%% is the ceiling.",
			pct, floor)
	}
}

func TestC331_005_LoadAnchorSHACorruptJSONCovered(t *testing.T) {
	const floor = 95.0
	out := coverFuncOutput(t, ledgerPkg)
	if pct := funcCoverage(t, out, "loadAnchorSHA"); pct < floor {
		t.Errorf("RED: ledger.loadAnchorSHA coverage %.1f%% < %.1f%% — add a test feeding corrupt "+
			"(non-JSON) anchor data and asserting it degrades to \"\". Baseline 85.7%%.", pct, floor)
	}
}

func TestC331_006_LedgerPackageCoverageFloor(t *testing.T) {
	const floor = 85.2
	out := coverFuncOutput(t, ledgerPkg)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/adapters/ledger coverage %.2f%% < %.1f%% — the seal + anchor "+
			"write-error tests must lift the package above the achievable floor. Baseline 84.29%%; "+
			"the scout's 87%% is unreachable via chmod (fd-level error arms cannot be injected).",
			pct, floor)
	}
}

func TestC331_007_LedgerSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", ledgerPkg)
	if code != 0 {
		t.Errorf("RED: internal/adapters/ledger suite fails under -race (exit=%d) — the new "+
			"write-error tests must not break the seal/anchor contract:\n%s", code, tail(combined, 30))
	}
}
