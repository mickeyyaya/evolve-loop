//go:build acs

// Package cycle1446 materialises the cycle-1446 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane (`context-fill-warn-threshold`).
// Both tasks close residual findings the cycle-1444 audit filed as WARN:
//
//   - contextfill-promptTokens-overflow-guard         → M1: PromptTokens sums
//     three driver-controlled counters with no overflow guard, so a wrapped sum
//     reaches FillPct and is published as a fabricated negative percentage that
//     is neither a real reading nor the documented FillPctUnmeasured sentinel.
//   - contextfill-acs-predicate-widen-run-pattern     → L3: the `-run` pattern
//     in the cycle-1444 ACS predicate and in the eval's evidence command
//     under-selects (Go `-run` is a substring match, and the word `Carry` breaks
//     `TestProductionDepsContextFill`), so the wiring-carries-through test is
//     silently skipped by both.
//
// Predicate strategy — every predicate exercises the system, never greps source
// (the cycle-85 degenerate-predicate ban):
//
//   - 001–002 call the real tokenusage API over the real overflow inputs, and
//     pin the anti-overfit half (honest large/over-full readings survive).
//   - 003 shells the task's own verifiableBy suite — ONE named package,
//     narrowed with -run, per the flaky-predicate-shape rules.
//   - 004–005 are the L3 predicates and are deliberately indirect: each READS
//     the `-run` pattern as literally written in the artifact under repair and
//     then EXECUTES it, asserting both wiring tests are selected. Asserting the
//     pattern string alone would be a grep; running the pattern the artifact
//     actually carries is the behaviour the criterion is about.
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

// claudeWindow is the conservative effective window for the claude family.
const claudeWindow = 200_000

// bridgeWiringTests are the two tests that must BOTH be selected by the
// evidence commands under repair. `Carry` in the first name is exactly what the
// narrow substring pattern misses.
var bridgeWiringTests = []string{
	"TestProductionDepsCarryContextFillThreshold",
	"TestProductionDepsContextFillRejectsOutOfRange",
}

// goDir returns the module root inside the cycle worktree.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// TestC1446_001_WrappedPromptTotalDegradesToSentinel — M1, the load-bearing
// predicate. Each counter below is individually a valid int that encoding/json
// lands in TokenUsage, but the sum wraps negative. The full PromptTokens→FillPct
// path must yield FillPctUnmeasured, never a fabricated negative percentage:
// that value is persisted verbatim to llm-calls.ndjson AND silently suppresses
// the WARN on exactly the launch whose telemetry is bogus.
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

// TestC1446_002_OverflowGuardKeepsHonestReadings — the anti-overfit half. The
// guard must not become a blanket "unusual ⇒ unmeasured" clamp: the documented
// invariant is that an over-full launch (120%) is a REAL signal and is not
// clamped away. Expected to be green before and after the fix; it exists so a
// lazy guard cannot green 001 by discarding honest readings.
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

// TestC1446_003_FillTelemetrySuiteIsGreen — the task's own verifiableBy. ONE
// named package narrowed with -run; the overflow subtests must pass alongside
// the pre-existing invariants (no regression on the sentinel or the
// over-full-is-not-clamped contract).
func TestC1446_003_FillTelemetrySuiteIsGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", "TestFillTelemetry", "./internal/tokenusage")
	if err != nil || code != 0 {
		t.Errorf("fill-telemetry suite is not green (exit=%d err=%v)\n%s\n%s", code, err, stdout, stderr)
	}
}

// runPatternFor extracts the `-run` pattern that `body` applies to the
// ./internal/adapters/bridge package, in either the Go-source form
// (`"-run", "X", "./internal/adapters/bridge"`) or the shell/eval form
// (`-run X ./internal/adapters/bridge`, optionally quoted).
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

// assertPatternSelectsBothWiringTests runs the given -run pattern against the
// real bridge adapter package and asserts BOTH wiring tests were selected and
// passed. This is the behavioural core of L3: an evidence command that selects
// only one of the two is blind to a real wiring regression.
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

// TestC1446_004_EvalEvidenceCommandSelectsBothWiringTests — L3, eval half. The
// score-cap evidence command is what caps a FUTURE cycle's audit score; if it
// under-selects, the cap is enforced against half the contract.
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

// TestC1446_005_ACSPredicateRunPatternSelectsBothWiringTests — L3, predicate
// half. Same defect in the cycle-1444 reachability predicate: it shells a
// pattern that matches only the out-of-range test, so the
// composition-root-carries-the-threshold half was never actually gated.
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
