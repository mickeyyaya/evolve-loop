//go:build acs

// Package cycle270 materializes the cycle-270 acceptance criteria for the
// `looppreflight-coverage-gaps` task: raise the lowest-coverage package in the
// tree (`internal/looppreflight`, 69.3% at the cycle-270 baseline) to >= 82% by
// unit-testing the six zero-coverage host-side `default*` seams and the
// `resolve()` nil-default branches. This is a TEST-ONLY change — no production
// logic moves — so the contract is "the suite gains these named tests, they
// pass, and coverage clears the bar."
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each RUNS the
// system-under-test — the `internal/looppreflight` Go suite — as a subprocess
// and asserts on its real `go test -cover -v` output (top-level + subtest PASS
// lines, the coverage %, absence of FAIL) and on a real `-race` run. There is
// NO source-grep gaming: a magic string in a .go file cannot produce a `--- PASS`
// line for a named test, nor move the coverage number. The builder's job is the
// new test code in `internal/looppreflight`; these predicates gate it.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	C1 → TestC270_001 (coverage >= 82%)
//	C2 → TestC270_002 (suite green under -race)
//	C3 → TestC270_003 (TestDefaultDirWritable: positive + negative)
//	C4 → TestC270_004 (TestDefaultTmuxSessions)
//	C5 → TestC270_005 (TestBootRCName: known + unknown codes)
//	C6 → TestC270_006 (TestResolve_NilDefaults)
//	C7 → manual+checklist (Auditor diff-scope review; see test-report.md)
package cycle270

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- one-shot runner: exercise the looppreflight suite once, share the output ---

var (
	lpOnce sync.Once
	lpOut  string
)

// runLoopPreflightSuite runs the full `internal/looppreflight` suite with
// coverage + verbose output ONCE per predicate process and returns combined
// stdout+stderr. `-C <goDir>` makes the invocation cwd-independent (the audit
// lane may run from the worktree root or go/); `-count=1` defeats the test cache
// so PASS lines and coverage reflect the builder's just-written files. The
// real-boot integration test is `//go:build integration`, excluded here, so the
// run stays hermetic.
func runLoopPreflightSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t) // t.Skip when not in a git work tree
	lpOnce.Do(func() {
		goDir := filepath.Join(root, "go")
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir, "-count=1", "-cover", "-v",
			"./internal/looppreflight/...")
		lpOut = stdout + "\n" + stderr
	})
	return lpOut
}

var (
	coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%\s+of statements`)
	// Top-level PASS lines are anchored at column 0; subtests are indented.
	topPassRe = regexp.MustCompile(`(?m)^--- PASS: (Test\w+)`)
	anyFailRe = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// parseCoverage extracts the reported statement coverage percentage, or -1 if
// the suite produced no coverage line (compile failure / no package).
func parseCoverage(out string) float64 {
	m := coverageRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return v
}

// topLevelPassed reports whether a column-0 `--- PASS: <name>` line is present.
func topLevelPassed(out, name string) bool {
	for _, m := range topPassRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

// subPasses returns the distinct passing subtest names under `parent`
// (indented `--- PASS: <parent>/<sub>` lines). Two distinct subtests prove a
// table-driven test exercised more than one row — the lever that forces the
// adversarial negative case alongside the positive (skills/adversarial-testing
// §6); a positive-only test is gameable by a no-op.
func subPasses(out, parent string) map[string]bool {
	re := regexp.MustCompile(`(?m)^\s+--- PASS: ` + regexp.QuoteMeta(parent) + `/(\S+)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	return seen
}

// --- C1: looppreflight package coverage >= 82% ---

// Load-bearing behavioral assertion: the coverage number is produced by REALLY
// running the suite over the package, so it can only clear 82% if the builder's
// new tests actually exercise the previously-uncovered `default*` seams and
// `resolve()` branches. RED at baseline (69.3%).
func TestC270_001_CoverageAtLeast82(t *testing.T) {
	out := runLoopPreflightSuite(t)
	cov := parseCoverage(out)
	if cov < 0 {
		t.Fatalf("RED: no `coverage: N%% of statements` line — suite did not build/run.\n%s", out)
	}
	if cov < 82.0 {
		t.Errorf("RED: looppreflight coverage = %.1f%%, want >= 82.0%% (baseline 69.3%%)", cov)
	}
}

// --- C2: the suite is green under the race detector ---

// Separate `-race` run (its own subprocess) so the cgo dependency is isolated to
// this predicate: a race-unsupported lane SKIPs rather than failing the whole
// gate. On a supported host, a non-zero exit with a FAIL line is a real race or
// test failure. (At RED time the new tests do not exist yet, so the existing
// suite passes under race — pre-existing GREEN; the value is post-GREEN: the
// builder's new tests must be race-clean.)
func TestC270_002_SuitePassesWithRace(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-race", "-count=1",
		"./internal/looppreflight/...")
	combined := stdout + "\n" + stderr
	if code != 0 {
		low := strings.ToLower(combined)
		if strings.Contains(low, "-race requires cgo") ||
			strings.Contains(low, "race detector not supported") ||
			strings.Contains(low, "requires cgo") {
			t.Skipf("race detector unsupported on this lane; skipping C2:\n%s", combined)
		}
		t.Errorf("RED: `go test -race ./internal/looppreflight/...` exit=%d (race or test failure)\n%s", code, combined)
	}
	if anyFailRe.MatchString(combined) {
		t.Errorf("RED: looppreflight suite has a FAIL line under -race\n%s", combined)
	}
}

// --- C3: defaultDirWritable is tested, positive AND negative ---

// `TestDefaultDirWritable` is named in the AC. Requiring it to PASS proves the
// real (formerly 0%) host probe is exercised; requiring >= 2 passing subtests
// forces BOTH a writable-dir positive and an unwritable/empty-dir negative — the
// negative is the anti-no-op signal (a stub `return true` would fail it).
func TestC270_003_DefaultDirWritableTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestDefaultDirWritable") {
		t.Errorf("RED: TestDefaultDirWritable did not run+PASS — defaultDirWritable (host.go) is still 0%% covered")
	}
	if subs := subPasses(out, "TestDefaultDirWritable"); len(subs) < 2 {
		t.Errorf("RED: TestDefaultDirWritable has %d passing sub-cases, want >= 2 (writable positive + unwritable/empty negative)", len(subs))
	}
}

// --- C4: defaultTmuxSessions is tested ---

// `TestDefaultTmuxSessions` exercises the real `tmux ls` adapter (host.go). The
// hermetic, deterministic case is the error path (no running server → nil slice
// + non-nil error), which is also the healthy-host case the caller relies on.
func TestC270_004_DefaultTmuxSessionsTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestDefaultTmuxSessions") {
		t.Errorf("RED: TestDefaultTmuxSessions did not run+PASS — defaultTmuxSessions (host.go) is still 0%% covered")
	}
}

// --- C5: bootRCName is tested, known AND unknown codes ---

// `TestBootRCName` must map the named bridge exit codes to their human strings
// and fall through to the default for an unrecognized code. Requiring >= 2
// passing subtests forces both a known-code row and the unknown-code negative
// (the default branch), so a switch missing its default cannot pass.
func TestC270_005_BootRCNameTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestBootRCName") {
		t.Errorf("RED: TestBootRCName did not run+PASS — bootRCName (boot.go) was only 33.3%% covered")
	}
	if subs := subPasses(out, "TestBootRCName"); len(subs) < 2 {
		t.Errorf("RED: TestBootRCName has %d passing sub-cases, want >= 2 (a known exit code + the unknown/default-branch case)", len(subs))
	}
}

// --- C6: resolve() nil-default branches are tested ---

// `TestResolve_NilDefaults` calls resolve() with a minimal Options (only the
// required ProjectRoot) and asserts every injectable function field is non-nil
// afterward — the binding logic (looppreflight.go:171, formerly 49%) that wires
// the real host implementations in. A missing nil-check would leave a field nil
// and the assertion would fire.
func TestC270_006_ResolveNilDefaultsTested(t *testing.T) {
	out := runLoopPreflightSuite(t)
	if !topLevelPassed(out, "TestResolve_NilDefaults") {
		t.Errorf("RED: TestResolve_NilDefaults did not run+PASS — resolve() nil-default branches (looppreflight.go:171) are still under-covered")
	}
}
