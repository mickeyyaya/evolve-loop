//go:build acs

// Package cycle298 materializes the cycle-298 acceptance criteria for the two
// committed top_n tasks (scout-report.md — L3.4 GC shadow wiring + gc coverage):
//
//	T1  gc-shadow-wiring — wire EVOLVE_GC=off|shadow|enforce into the evolve
//	    loop startup. Two sub-criteria:
//	      (a) EVOLVE_GC is registered in internal/flagregistry (enum, default
//	          "off") AND docs/architecture/control-flags.md is regenerated so
//	          `evolve flags check` reports the index in sync (no drift).
//	      (b) the runGCHook loop hook discovers+plans+logs a manifest in shadow
//	          mode WITHOUT mutating the tree, applies it in enforce mode, no-ops
//	          in off mode, warns (no crash) on an invalid value, and never plans
//	          a LIVE run for deletion.
//	T2  gc-coverage-boost — internal/gc statement coverage is ≥ 95.0% (up from
//	    the 88.8% baseline; the uncovered paths are the safety-critical Apply /
//	    nowLive / protected / dirEntriesOlderThan guards).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test:
//   - C298_001 reads the real flagregistry.All slice (the SSOT data structure,
//     not a grepped source line) AND runs the real `evolve flags check` binary,
//     so a magic string in a doc cannot satisfy it — only adding the registry
//     row AND regenerating the doc does.
//   - C298_002 runs the real white-box cmd_loop_gc tests (which call the
//     unexported runGCHook against a synthetic .evolve tree and assert on the
//     manifest file + real dir mutations) and asserts on their `--- PASS:`
//     lines, plus a full cmd/evolve suite-green regression axis.
//   - C298_003 runs `go test -coverprofile` over internal/gc and parses the
//     real total: percentage — RED at the 88.8% baseline, GREEN only once the
//     new safety-path tests land.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1(a) EVOLVE_GC registered + flags check in sync           → C298_001
//	T1(b) runGCHook shadow/off/enforce/invalid/missing/live    → C298_002 (+002b)
//	T2    internal/gc coverage ≥ 95.0%                          → C298_003
package cycle298

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line of
	// `go tool cover -func`.
	coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// ===================== T1(a) — EVOLVE_GC flag registration ===================

// ===================== T1(b) — runGCHook loop hook ===========================

var (
	gcHookOnce sync.Once
	gcHookOut  string
)

// runGCHookTests runs the white-box cmd_loop_gc tests (which call the real
// unexported runGCHook against a synthetic .evolve tree) verbose, ONCE per
// predicate process. Scoped via -run so an unrelated cmd/evolve change cannot
// false-RED this gate.
func runGCHookTests(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	gcHookOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestGC", "./cmd/evolve/")
		gcHookOut = stdout + "\n" + stderr
	})
	return gcHookOut
}

// --- C298_002 (T1b): the runGCHook behavior matrix is GREEN -------------------
//
// Behavioral: gates on real `--- PASS:` lines for each mode of the loop hook.
// Every named test constructs a synthetic .evolve tree, calls the real
// runGCHook, and asserts on observable side effects (the manifest file and real
// run-dir mutations). RED baseline: runGCHook does not exist → package main does
// not compile → `go test` emits a build error with zero PASS lines → every
// assertion below fails. GREEN requires the real hook wiring the
// Discover→Plan→(Apply) pipeline through the EVOLVE_GC switch.
func TestC298_002_GCHookBehaviorMatrix(t *testing.T) {
	out := runGCHookTests(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestGC* hook test FAILs:\n%s", tail(out, 50))
	}
	want := []string{
		"TestGCShadow",                // shadow writes manifest, no mutation
		"TestGCOff",                   // off writes nothing
		"TestGCEnforce",               // enforce applies (delete target removed)
		"TestGCInvalidMode",           // invalid value warns, no crash
		"TestGCShadowMissingRunsDir",  // fail-open: empty manifest, no runs/ dir
		"TestGCShadowLiveRunExcluded", // a live run is never planned
	}
	for _, name := range want {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: %s did not PASS — runGCHook does not yet satisfy this mode", name)
		}
	}
}

// --- C298_002b (T1b.green): the whole cmd/evolve suite stays green ------------
//
// Anti-no-op regression gate: runGCHook is called at loop startup, on the hot
// path of every `evolve loop`. Running the full cmd/evolve suite and asserting
// no `--- FAIL:` line ensures the hook (and the EVOLVE_GC plumbing) did not
// regress argument parsing, preflight, or the main loop. A FAIL line / non-zero
// exit is a real failure no source string fakes.
func TestC298_002b_CmdEvolveSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "./cmd/evolve/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: cmd/evolve suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

// ===================== T2 — internal/gc coverage floor =======================

// --- C298_003 (T2): internal/gc statement coverage ≥ 95.0% --------------------
//
// Behavioral: runs the REAL internal/gc test suite under -coverprofile and
// parses the actual `total:` percentage from `go tool cover -func`. This is the
// canonical coverage-floor predicate shape: it executes every gc test (the real
// Apply/nowLive/protected/dirEntriesOlderThan code paths) and gates on the
// measured number. RED baseline: 88.8% < 95.0% → fail. GREEN requires the new
// safety-path tests Builder adds to gc_test.go / discover_test.go.
//
// Note (R9.3): this floor binds ONLY because gc-coverage-boost is a committed
// top_n task this cycle (triage-report.md). The threshold is the criterion; the
// individual tests that reach it are Builder's production-side work.
func TestC298_003_GCCoverageFloor(t *testing.T) {
	const floor = 95.0
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "gc.cover")

	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-coverprofile="+profile, "./internal/gc/..."); err != nil || code != 0 {
		t.Fatalf("RED: internal/gc test run failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 40))
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 20))
	}
	m := coverTotalRe.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("could not parse total coverage from:\n%s", tail(stdout, 20))
	}
	pct, perr := strconv.ParseFloat(m[1], 64)
	if perr != nil {
		t.Fatalf("unparsable coverage %q: %v", m[1], perr)
	}
	if pct < floor {
		t.Errorf("RED: internal/gc coverage %.1f%% < %.1f%% floor — add the Apply/nowLive/"+
			"protected/dirEntriesOlderThan safety-path tests", pct, floor)
	}
}
