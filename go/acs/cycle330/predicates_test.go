//go:build acs

package cycle330

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
	interactionPkg = "./internal/interaction/..."
	coherencePkg   = "./internal/phasecoherence/..."
	bridgePkg      = "./internal/bridge/"
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

func TestC330_001_BridgeBareDiffNoEscalate(t *testing.T) {
	combined, code := runGoTest(t, "-v", "-count=1", "-run", "TestDecideAutoRespond", bridgePkg)
	if code != 0 {
		t.Errorf("RED: decideAutoRespond suite fails (exit=%d) — a bare unified-diff line "+
			"(\"+content\"/\"-content\", no leading line number) carrying banner text on an IDLE pane "+
			"must be stripped before escalate-pattern matching; a real banner must still escalate:\n%s",
			code, tail(combined, 40))
	}
	const passLine = "--- PASS: TestDecideAutoRespond_BareDiffLineNotChrome"
	if !strings.Contains(combined, passLine) {
		t.Errorf("RED: %q not present — the bare-diff exclusion test did not PASS (failing, renamed, or removed):\n%s",
			passLine, tail(combined, 40))
	}
}

func TestC330_002_LedgerPathEmptyPhaseCovered(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "ledgerPath"); pct < floor {
		t.Errorf("RED: interaction.ledgerPath coverage %.1f%% < %.1f%% — add a test driving the "+
			"empty-phase \"unknown\" fallback. Baseline 66.7%%.", pct, floor)
	}
}

func TestC330_003_NeutralizeMultiLineRuneCapCovered(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "neutralize"); pct < floor {
		t.Errorf("RED: interaction.neutralize coverage %.1f%% < %.1f%% — add a multi-line payload "+
			"(>200 joined runes) that triggers the second rune-cap. Baseline 83.3%%.", pct, floor)
	}
}

func TestC330_004_AppendLedgerLineOpenFileErrorCovered(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t, interactionPkg)
	if pct := funcCoverage(t, out, "appendLedgerLine"); pct < floor {
		t.Errorf("RED: interaction.appendLedgerLine coverage %.1f%% < %.1f%% — add a test that drives "+
			"os.OpenFile to error (parent path is a file, not a dir). Baseline 77.8%%; the marshal-error "+
			"arm is unreachable so ~88.9%% is the ceiling.", pct, floor)
	}
}

func TestC330_005_InteractionSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", interactionPkg)
	if code != 0 {
		t.Errorf("RED: internal/interaction suite fails under -race (exit=%d) — the new error-branch "+
			"tests must not break the ledger/neutralize contract:\n%s", code, tail(combined, 30))
	}
}

func TestC330_006_CheckNilGuardBranchesCovered(t *testing.T) {
	const floor = 88.0
	out := coverFuncOutput(t, coherencePkg)
	if pct := funcCoverage(t, out, "Check"); pct < floor {
		t.Errorf("RED: phasecoherence.Check coverage %.1f%% < %.1f%% — add tests for nil AgentsFS, "+
			"directory-entry skip, nil-frontmatter skip, and non-slice toolsVal skip. Baseline 81.4%%.",
			pct, floor)
	}
}

func TestC330_007_PhasecoherenceSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", coherencePkg)
	if code != 0 {
		t.Errorf("RED: internal/phasecoherence suite fails under -race (exit=%d) — the new nil-guard "+
			"tests must not break the coherence-check contract:\n%s", code, tail(combined, 30))
	}
}
