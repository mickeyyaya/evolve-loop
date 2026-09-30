//go:build acs

package cycle1446

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const claudeWindow = 200_000

var bridgeWiringTests = []string{
	"TestProductionDepsCarryContextFillThreshold",
	"TestProductionDepsContextFillRejectsOutOfRange",
}

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func TestC1446_001_WrappedPromptTotalDegradesToSentinel(t *testing.T) {
	const maxInt = int(^uint(0) >> 1)
	cases := []struct {
		name  string
		usage cyclestate.TokenUsage
	}{
		{"input+cacheRead wraps past MaxInt", cyclestate.TokenUsage{Input: maxInt, CacheRead: 1}},
		{"three halves wrap", cyclestate.TokenUsage{Input: maxInt/2 + 1, CacheRead: maxInt/2 + 1, CacheWrite: 2}},
		{"a negative counter cannot fabricate a reading", cyclestate.TokenUsage{Input: -500_000, CacheRead: 1_000}},
	}
	for _, c := range cases {
		pct := tokenusage.FillPct(tokenusage.PromptTokens(c.usage), claudeWindow)
		if pct != tokenusage.FillPctUnmeasured {
			t.Errorf("%s: FillPct(PromptTokens(%+v), %d) = %v, want the FillPctUnmeasured sentinel (%v) — a wrapped total must degrade to unmeasured, not to a fabricated percentage",
				c.name, c.usage, claudeWindow, pct, tokenusage.FillPctUnmeasured)
		}
		if warn := tokenusage.FillWarn("build", pct, 60); warn != "" {
			t.Errorf("%s: FillWarn on the sentinel = %q, want silence", c.name, warn)
		}
	}
}

func TestC1446_002_OverflowGuardKeepsHonestReadings(t *testing.T) {
	cases := []struct {
		name  string
		usage cyclestate.TokenUsage
		want  float64
	}{
		{"ordinary launch", cyclestate.TokenUsage{Input: 100_000, CacheRead: 20_000, Output: 9_999_999}, 60},
		{"empty prompt is a measured zero", cyclestate.TokenUsage{Output: 1_000}, 0},
		{"over-full is still not clamped", cyclestate.TokenUsage{Input: 200_000, CacheWrite: 40_000}, 120},
	}
	for _, c := range cases {
		got := tokenusage.FillPct(tokenusage.PromptTokens(c.usage), claudeWindow)
		if got < c.want-0.001 || got > c.want+0.001 {
			t.Errorf("%s: FillPct(PromptTokens(%+v), %d) = %v, want %v — the overflow guard must not swallow honest readings",
				c.name, c.usage, claudeWindow, got, c.want)
		}
	}
}

func TestC1446_003_FillTelemetrySuiteIsGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "TestFillTelemetry", "./internal/tokenusage")
	if err != nil || code != 0 {
		t.Errorf("fill-telemetry suite is not green (exit=%d err=%v)\n%s\n%s", code, err, stdout, stderr)
	}
}

func runPatternFor(body string) (string, bool) {
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`"-run",\s*"([^"]+)",\s*"\./internal/adapters/bridge"`),
		regexp.MustCompile(`-run\s+['"]?([^\s'"]+)['"]?\s+(?:-v\s+)?\./internal/adapters/bridge`),
	} {
		if m := re.FindStringSubmatch(body); m != nil {
			return m[1], true
		}
	}
	return "", false
}

func assertPatternSelectsBothWiringTests(t *testing.T, source, pattern string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", pattern, "-v", "./internal/adapters/bridge")
	if err != nil || code != 0 {
		t.Errorf("%s: `-run %s` did not pass (exit=%d err=%v)\n%s\n%s", source, pattern, code, err, stdout, stderr)
		return
	}
	for _, name := range bridgeWiringTests {
		if !strings.Contains(stdout, "=== RUN   "+name) {
			t.Errorf("%s: `-run %s` never selected %s — Go's -run is a substring match, so this evidence command silently skips it and is blind to a wiring regression\nstdout:\n%s",
				source, pattern, name, stdout)
		}
	}
}

func TestC1446_004_EvalEvidenceCommandSelectsBothWiringTests(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", "context-fill-warn-threshold.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the eval under repair: %v", err)
	}
	pattern, ok := runPatternFor(string(body))
	if !ok {
		t.Fatalf("%s: no `-run <pattern> ./internal/adapters/bridge` evidence command found — the policy-reaches-deps score cap lost its evidence", path)
	}
	assertPatternSelectsBothWiringTests(t, "eval evidence command", pattern)
}

func TestC1446_005_ACSPredicateRunPatternSelectsBothWiringTests(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go", "acs", "cycle1444", "predicates_test.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the cycle-1444 predicate under repair: %v", err)
	}
	pattern, ok := runPatternFor(string(body))
	if !ok {
		t.Fatalf("%s: no `-run` pattern targeting ./internal/adapters/bridge found — the reachability predicate lost its subject", path)
	}
	assertPatternSelectsBothWiringTests(t, "cycle-1444 ACS predicate", pattern)
}
