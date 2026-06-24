//go:build acs

// Package cycle322 materializes the cycle-322 acceptance criteria for the one
// committed top_n task (scout-report.md "## Selected Tasks"):
//
//	modelcatalog-write-error-paths — raise go/internal/modelcatalog statement
//	    coverage from the 87.3% baseline to the committed >= 92.0% floor by
//	    exercising the dark temp-file error exits of store.Write (66.7% — the
//	    mkdir / CreateTemp / Rename exits are ALREADY covered by
//	    store_errors_test.go; the remaining dark branches are tmp.Write,
//	    tmp.Sync, and tmp.Close, which need an injectable seam over
//	    os.CreateTemp). The json.MarshalIndent error arm is unreachable for a
//	    plain Catalog struct, so 85% — not 100% — is the Write ceiling.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). C322_001/002 RUN the real
// modelcatalog suite in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; C322_003/004 RUN the suite and
// assert on its exit code; C322_005 CALLS the real modelcatalog.Write through
// its public API on a rename-failing path and asserts on the returned error and
// the absence of a leaked temp file. There is no load-bearing source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's createTemp seam + the new tmp.Write/Sync/Close failure tests can.
//   - An EMPTY repo (no modelcatalog tests) yields 0% and fails every floor, so
//     the coverage gates are anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC4). The
//     new store_writefail_test.go references the not-yet-existing createTemp seam,
//     so today the package does not even COMPILE — every coverage/suite gate is
//     RED until Builder adds the seam.
//   - C322_005 is the explicit behavioral negative axis (adversarial-testing
//     SKILL §6): it drives Write's rename error exit and requires both a non-nil
//     error AND a clean directory — a happy-path-only Write cannot satisfy it.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 5 ACs):
//
//	modelcatalog-write-error-paths
//	  AC1 package coverage >= 92.0%                         → C322_001
//	  AC2 Write coverage >= 85%                             → C322_002
//	  AC3 read-only-dir write error test passes             → C322_003
//	  AC4 suite green (no regression)                       → C322_004
//	  AC5 no temp files left on failure paths               → C322_005
//
// AC3 note: scout-report.md AC3 names a test "TestWriteReadOnlyDirError"; the
// pre-existing test that pins exactly that behavior (a read-only evolveDir making
// CreateTemp fail) is TestWriteCreateTempFailsInReadOnlyDir in store_errors_test.go.
// AC3 binds to that real test name rather than churning the file with a rename.
//
// Floor binding (R9.3): internal/modelcatalog is the committed top_n task this
// cycle, so the coverage floors bind a committed package. The scout-DEFERRED
// items (ledger seal, cmd/evolve) get ZERO predicates here — authoring a floor on
// a deferred task would starve the committed one (cycle-280 lesson).
package cycle322

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const modelcatalogPkg = "./internal/modelcatalog/..."

// tail returns the last n lines of s (keeps RED failure messages readable).
func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// runGoTest runs `go test -C <goDir> <args...>` via the shared acsassert helper
// and returns the combined output and exit code. It does not Fatal on a non-zero
// test exit (callers assert on that); acsassert.SubprocessOutput only errors when
// the toolchain itself fails to launch.
func runGoTest(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	full := append([]string{"test", "-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, tail(stderr, 30))
	}
	return stdout + stderr, code
}

// coverTotalRe matches the trailing `total:  (statements)  NN.N%` line.
var coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)

// coverFuncOutput runs the REAL modelcatalog suite under -coverprofile and
// returns the `go tool cover -func` report. Fatals (RED) if the suite does not
// compile or a test FAILs — a coverage number is only meaningful over a green
// suite, so this folds in the no-regression axis (AC4).
func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, modelcatalogPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the createTemp seam + the "+
			"tmp.Write/Sync/Close failure tests / fix regressions:\n%s", modelcatalogPkg, code, tail(combined, 40))
	}
	stdout, stderr, code2, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if err != nil || code2 != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code2, err, tail(stderr, 20))
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
// `<file>:<line>:\t<fn>\t<pct>%` lines; Fatals if the function is absent. The
// `:\d+:` anchor binds fn to a real func-coverage row (not a substring of
// another symbol).
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

// ================ modelcatalog-write-error-paths predicates ==================

// --- C322_001 (AC1): go/internal/modelcatalog total coverage >= 92.0% --------
//
// Behavioral coverage-floor over the real modelcatalog suite. RED baseline:
// 87.3% (and today the package does not compile until the createTemp seam lands,
// which makes coverFuncOutput Fatal). GREEN requires Builder's createTemp seam +
// the tmp.Write/Sync/Close failure tests. The shared green-suite gate in
// coverFuncOutput Fatals if any existing test regresses, so this folds in the
// no-regression axis too.
func TestC322_001_ModelcatalogCoverageFloor(t *testing.T) {
	const floor = 92.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/modelcatalog coverage %.1f%% < %.1f%% floor — exercise store.Write's "+
			"dark tmp.Write / tmp.Sync / tmp.Close error exits via the createTemp seam. Baseline 87.3%%.", pct, floor)
	}
}

// --- C322_002 (AC2): store.Write coverage >= 85% -----------------------------
//
// Behavioral + negative axis. Write is at 66.7% today: the mkdir / CreateTemp /
// Rename exits are already covered by store_errors_test.go, but the three
// temp-file exits — tmp.Write, tmp.Sync, tmp.Close — are dark and need an
// injectable seam. (The json.MarshalIndent error arm is unreachable for a plain
// Catalog, which is why the floor is 85%, not 100%.) Reaching 85% REQUIRES tests
// that drive Write through its temp-file write/sync/close failures.
func TestC322_002_StoreWriteCoverageFloor(t *testing.T) {
	const floor = 85.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "Write"); pct < floor {
		t.Errorf("RED: store.Write coverage %.1f%% < %.1f%% — add tests that fail tmp.Write, tmp.Sync, "+
			"and tmp.Close via the createTemp seam. Baseline 66.7%%; the marshal-error arm is unreachable "+
			"so ~96%% (not 100%%) is the practical ceiling.", pct, floor)
	}
}

// --- C322_003 (AC3): the read-only-dir CreateTemp-fail test passes -----------
//
// Behavioral regression anchor. Runs the pre-existing
// TestWriteCreateTempFailsInReadOnlyDir (store_errors_test.go) which makes
// evolveDir read-only so os.CreateTemp fails with EACCES and Write must surface
// "modelcatalog: tempfile". This is scout AC3 ("read-only dir write error").
// RED today only because the package fails to compile without the seam; GREEN
// once the seam lands and the suite compiles.
func TestC322_003_ReadOnlyDirWriteErrorTestPasses(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestWriteCreateTempFailsInReadOnlyDir",
		"-count=1", modelcatalogPkg)
	if code != 0 {
		t.Errorf("RED: TestWriteCreateTempFailsInReadOnlyDir does not pass (exit=%d) — the read-only-dir "+
			"write-error contract must hold (and the package must compile):\n%s", code, tail(combined, 30))
	}
}

// --- C322_004 (AC4): the existing modelcatalog contract suite stays green ----
//
// Regression axis. Runs the pre-existing TestRead* / TestWriteThenReadRoundTrip /
// TestWriteCreates* / TestWriteIsAtomic* / TestLookup* / TestBuildFromSnapshots
// contract suite. Builder must KEEP it green while adding the new failure-path
// tests. Uses a distinct -run command verb from the coverage gate for lexical
// diversity.
func TestC322_004_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run",
		"TestRead|TestWriteThenReadRoundTrip|TestWriteCreates|TestWriteIsAtomic|TestLookup|TestBuildFromSnapshots|TestDispatch",
		"-count=1", modelcatalogPkg)
	if code != 0 {
		t.Errorf("RED: the existing modelcatalog contract suite fails (exit=%d) — the new temp-file "+
			"error-path tests must not break the round-trip / atomic-write / lookup contract:\n%s",
			code, tail(combined, 30))
	}
}

// --- C322_005 (AC5): no temp file leaks on a failed write --------------------
//
// Behavioral negative axis (adversarial-testing SKILL §6). Drives the real
// modelcatalog.Write through its rename error exit: a NON-EMPTY directory
// occupies the final catalog path, so os.Rename(temp, catalog) fails and Write
// must (a) return a non-nil error AND (b) leave no *.tmp behind (the cleanup()
// best-effort removal). This CALLS the system under test (no source-grep) and a
// happy-path-only Write — or one that forgot the cleanup — could not satisfy it.
// Pre-existing GREEN (the rename-fail cleanup already works); it anchors AC5 as
// an explicit behavioral contract.
func TestC322_005_NoTempLeakOnFailedWrite(t *testing.T) {
	dir := t.TempDir()
	// Occupy the final catalog path with a non-empty directory: renaming the temp
	// FILE over a non-empty DIR fails on every supported platform.
	target := filepath.Join(dir, modelcatalog.FileName)
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o755); err != nil {
		t.Fatalf("arrange: mkdir target-as-nonempty-dir: %v", err)
	}

	err := modelcatalog.Write(dir, modelcatalog.Catalog{})
	if err == nil {
		t.Fatalf("RED: Write onto a non-empty dir must return a non-nil rename error")
	}

	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatalf("ReadDir after failed write: %v", rerr)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("RED: temp file leaked after failed write: %s — cleanup() must remove it", e.Name())
		}
	}
}
