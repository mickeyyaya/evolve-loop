//go:build acs

// Package cycle316 materializes the cycle-316 acceptance criteria for the single
// committed top_n task (scout-report.md "Selected Tasks"):
//
//	clihealth-zero-coverage — add unit tests for the four zero-coverage functions
//	    in go/internal/clihealth/clihealth.go (Benchable, NewBenchEntry, firstLine,
//	    truncateRunes), lifting package statement coverage from the 76.2% baseline
//	    to >= 90%. The four functions are the bench-record composition core: the
//	    runner's bench-writer and the loop's canary both compose entries via
//	    NewBenchEntry (→ firstLine → truncateRunes), and llmroute consults Benchable
//	    to decide whether a classified wall benches the whole CLI family. They were
//	    the only 0.0% rows in `go tool cover -func` for this package.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298/299/300: the coverage gates RUN the real
// internal/clihealth test suite under -coverprofile (a real subprocess exercising
// real code paths) and assert on the measured percentage from `go tool cover
// -func`. A magic string in a source file cannot move the number — only Builder's
// new tests, which actually call the functions, can. An EMPTY repo (no tests)
// yields 0% and fails every floor, so they are anti-no-op by construction. The
// negative-contract gate (C316_004) imports clihealth and calls Benchable directly
// — the strongest behavioral anti-no-op for a one-statement predicate that
// coverage alone cannot force.
//
// coverFuncOutput Fatals (RED) if the clihealth suite does not compile or any test
// FAILs, so every coverage gate folds in the no-regression axis (AC4).
//
// AC map (1:1 with scout-report.md Task 1 "Acceptance criteria", 4 ACs):
//
//	AC1 coverage >= 90%                          → C316_001 (+ green-suite gate = AC4)
//	AC2 none of the 4 functions at 0.0%          → C316_002
//	AC3 edge case (firstLine empty / truncate    → C316_003 (firstLine=100% ∧
//	    multi-byte rune boundary)                       truncateRunes=100%, branch-forcing)
//	AC3 negative case (Benchable rejects          → C316_004 (direct Benchable call)
//	    non-rate_limit)
//	AC4 existing tests still pass                  → green-suite gate shared by
//	                                                  C316_001/002/003 (DRY — no
//	                                                  duplicate suite run)
//
// Floor binding (R9.3): internal/clihealth is the ONLY committed top_n task this
// cycle, so the coverage floor binds a committed package. The triage-deferred
// adapters/ledger seal.go coverage target (scout "Deferred") gets ZERO predicates
// here — authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
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

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const clihealthPkg = "./internal/clihealth/..."

// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line of
// `go tool cover -func`.
var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// coverFuncOutput runs the REAL clihealth test suite under -coverprofile and
// returns the `go tool cover -func` report. Fatals (RED) if the suite does not
// compile or a test FAILs — a coverage number is only meaningful over a green
// suite, so this also folds in the no-regression axis (AC4).
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

// totalCoverage parses the `total:` percentage; Fatals if absent.
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

// funcCoverage parses the per-function percentage for fn from the
// `<file>:<line>:\t<fn>\t<pct>%` lines; Fatals if the function is absent.
// The `:\d+:` anchor binds fn to a real func-coverage row (not a substring of
// another symbol) — e.g. searching "firstLine" never matches some "firstLineFoo".
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

// ===================== clihealth-zero-coverage predicates =====================

// --- C316_001 (AC1 + AC4): internal/clihealth total coverage >= 90.0% ---------
//
// Behavioral coverage-floor over the real clihealth suite. RED baseline: 76.2%.
// GREEN requires Builder's new tests for all four 0.0% functions — measurement
// shows that fully covering Benchable, NewBenchEntry (BOTH the cooldown and the
// reset-hint branch), firstLine, and truncateRunes lifts the package to exactly
// 90.1%, so the 90.0% floor leaves ~0.1% of margin and effectively REQUIRES every
// one of the four to be completely exercised (a partially-covered NewBenchEntry,
// e.g. skipping the ParseResetHint path, drops the total below 90%). The shared
// green-suite gate in coverFuncOutput Fatals if any existing test regresses (AC4).
func TestC316_001_ClihealthCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/clihealth coverage %.1f%% < %.1f%% floor — fully exercise "+
			"Benchable, NewBenchEntry (both the cooldown AND the reset-hint branch), firstLine, "+
			"and truncateRunes", pct, floor)
	}
}

// --- C316_002 (AC2): none of the four target functions remain at 0.0% ---------
//
// Behavioral per-function floor. The four functions are dark today (all 0.0% in
// `go tool cover -func`); AC2 requires every one to be exercised by at least one
// real test. A magic string cannot lift a function's coverage row above 0% — only
// a test that actually calls it can. RED baseline: all four at 0.0%.
func TestC316_002_ZeroCoverageFunctionsExercised(t *testing.T) {
	out := coverFuncOutput(t)
	for _, fn := range []string{"Benchable", "NewBenchEntry", "firstLine", "truncateRunes"} {
		if pct := funcCoverage(t, out, fn); pct <= 0.0 {
			t.Errorf("RED: clihealth.%s coverage %.1f%% — still at zero, AC2 requires it exercised", fn, pct)
		}
	}
}

// --- C316_003 (AC3 edge case): firstLine = 100% AND truncateRunes = 100% ------
//
// Behavioral per-function floor encoding AC3's edge-case requirement. Both
// functions are two-branch: firstLine returns s[:i] when a '\n' is found else the
// whole string (the empty-string / no-newline path AC3 names), and truncateRunes
// returns s when len(runes) <= n else string(r[:n]) (the multi-byte rune-boundary
// truncation AC3 names). A function reaches 100% statement coverage ONLY when BOTH
// of its branches are driven, so a 100% floor on each is the precise behavioral
// encoding of "tests include the empty-string firstLine edge AND the multi-byte
// truncateRunes boundary edge" — a single happy-path call cannot satisfy it. RED
// baseline: both at 0.0%.
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

// --- C316_004 (AC3 negative case): Benchable rejects non-rate_limit patterns ---
//
// Behavioral contract pin. AC3 requires a negative case (Benchable returns false
// for a non-rate_limit pattern), but Benchable is a single `return pattern ==
// "rate_limit"` statement — one call covers it 100%, so coverage alone cannot
// force the negative assertion. This predicate instead CALLS the real Benchable
// directly and asserts its full positive+negative contract: rate_limit benches,
// auth_recheck (the documented next-candidate) and the empty string do not. This
// is the GOOD pattern (exercises the system under test) and permanently pins the
// closed-set contract so a future widening of Benchable cannot silently bench
// situational escalations. Pre-existing GREEN — Benchable already behaves
// correctly — so it gates the contract rather than the Builder's RED→GREEN work.
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
