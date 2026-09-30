//go:build acs

package cycle316

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const clihealthPkg = "./internal/clihealth/..."

var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func coverFuncOutput(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "c.cover")
	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-coverprofile="+profile, clihealthPkg); err != nil || code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d, err=%v) — add the new tests / fix regressions:\n%s",
			clihealthPkg, code, err, tail(stderr, 40))
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

func TestC316_001_ClihealthCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/clihealth coverage %.1f%% < %.1f%% floor — fully exercise "+
			"Benchable, NewBenchEntry (both the cooldown AND the reset-hint branch), firstLine, "+
			"and truncateRunes", pct, floor)
	}
}

func TestC316_002_ZeroCoverageFunctionsExercised(t *testing.T) {
	out := coverFuncOutput(t)
	for _, fn := range []string{"Benchable", "NewBenchEntry", "firstLine", "truncateRunes"} {
		if pct := funcCoverage(t, out, fn); pct <= 0.0 {
			t.Errorf("RED: clihealth.%s coverage %.1f%% — still at zero, AC2 requires it exercised", fn, pct)
		}
	}
}

func TestC316_003_EdgeCaseBranchesCovered(t *testing.T) {
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "firstLine"); pct < 100.0 {
		t.Errorf("RED: clihealth.firstLine coverage %.1f%% < 100%% — cover BOTH the newline-found "+
			"path and the no-newline/empty-string path (the AC3 empty-string edge)", pct)
	}
	if pct := funcCoverage(t, out, "truncateRunes"); pct < 100.0 {
		t.Errorf("RED: clihealth.truncateRunes coverage %.1f%% < 100%% — cover BOTH the below-limit "+
			"path and the multi-byte-rune truncation path (the AC3 rune-boundary edge)", pct)
	}
}

func TestC316_004_BenchableNegativeContract(t *testing.T) {
	if !clihealth.Benchable("rate_limit") {
		t.Error("Benchable(\"rate_limit\") = false, want true — rate_limit is the one benchable pattern")
	}
	for _, pat := range []string{"auth_recheck", "quota", "trust_prompt", ""} {
		if clihealth.Benchable(pat) {
			t.Errorf("Benchable(%q) = true, want false — only rate_limit may bench a whole CLI family", pat)
		}
	}
}
