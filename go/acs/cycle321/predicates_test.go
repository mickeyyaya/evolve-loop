//go:build acs

// Package cycle321 materializes the cycle-321 acceptance criteria for the one
// committed top_n task (triage-report.md / scout-report.md "## Selected Tasks"):
//
//	clihealth-store-io-coverage — raise go/internal/clihealth statement coverage
//	    from the 90.1% baseline to the committed >= 94.0% floor by exercising the
//	    dark I/O error paths of the persistent bench Store: Store.write (63.6% —
//	    the mkdir-fail, WriteFile-fail and Rename-fail exits are dark; only the
//	    happy round-trip is hit), NewStore (66.7% — the nil-clock default arm
//	    `now = time.Now` is never taken because every test injects a fixed clock),
//	    and Store.Load (76.9% — the read-error WARN degradation and the
//	    valid-JSON-but-nil-`benches` arm are dark), while every existing
//	    clihealth test stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). C321_001..005 RUN the real
// clihealth test suite in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; C321_006 CALLS the real Store via
// its exported Bench entry point and asserts on the returned error. There is no
// load-bearing source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests, which actually drive write through its error exits,
//     call NewStore(root, nil), and feed Load a corrupt / typed / unreadable
//     file, can.
//   - An EMPTY repo (no clihealth tests) yields 0% and fails every floor, so the
//     coverage gates are anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC5).
//   - C321_006 is the explicit ADVERSARIAL negative axis (adversarial-testing
//     SKILL §6): it drives write's first error exit through the public Bench API
//     and requires a non-nil error — a happy-path-only suite cannot satisfy it.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria Summary" — 6 ACs):
//
//	clihealth-store-io-coverage
//	  AC1 package coverage >= 94.0%                      → C321_001
//	  AC2 write coverage >= 90%                          → C321_002
//	  AC3 NewStore coverage >= 90%                       → C321_003
//	  AC4 Load coverage >= 90%                           → C321_004
//	  AC5 suite green (no regression)                    → C321_005
//	  AC6 negative: write on read-only root → non-nil err → C321_006
//
// Floor binding (R9.3): internal/clihealth is the committed top_n task this
// cycle, so the coverage floors bind a committed package. The scout-DEFERRED
// items (ledger seal, modelcatalog Write, cmd/evolve) get ZERO predicates here —
// authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
package cycle321

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

const clihealthPkg = "./internal/clihealth/..."

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

// coverFuncOutput runs the REAL clihealth suite under -coverprofile and returns
// the `go tool cover -func` report. Fatals (RED) if the suite does not compile
// or a test FAILs — a coverage number is only meaningful over a green suite, so
// this folds in the no-regression axis (AC5).
func coverFuncOutput(t *testing.T) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "c.cover")
	combined, code := runGoTest(t, "-count=1", "-coverprofile="+profile, clihealthPkg)
	if code != 0 {
		t.Fatalf("RED: %s test run failed (exit=%d) — add the write/NewStore/Load I/O error-path "+
			"tests / fix regressions:\n%s", clihealthPkg, code, tail(combined, 40))
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

// ================ clihealth-store-io-coverage predicates ==================

// --- C321_001 (AC1): go/internal/clihealth total coverage >= 94.0% -----------
//
// Behavioral coverage-floor over the real clihealth suite. RED baseline: 90.1%.
// GREEN requires Builder's new write/NewStore/Load error-path tests. The shared
// green-suite gate in coverFuncOutput Fatals if any existing test regresses, so
// this folds in the no-regression axis too. Floor pinned at the triage-committed
// 94.0% (scout-report AC1) — Builder may deliver higher.
func TestC321_001_ClihealthCoverageFloor(t *testing.T) {
	const floor = 94.0
	out := coverFuncOutput(t)
	if pct := totalCoverage(t, out); pct < floor {
		t.Errorf("RED: internal/clihealth coverage %.1f%% < %.1f%% floor — exercise the dark write "+
			"error exits, NewStore nil-clock default, and Load degradation arms. Baseline 90.1%%.", pct, floor)
	}
}

// --- C321_002 (AC2): Store.write coverage >= 90% ------------------------------
//
// Behavioral + ADVERSARIAL negative axis. write is at 63.6% today: only the
// happy round-trip is exercised. The three error exits — os.MkdirAll fail,
// os.WriteFile fail, os.Rename fail — are dark. (The json.MarshalIndent error
// arm is unreachable for a map[string]Entry, which is why the floor is 90%, not
// 100%.) Reaching 90% REQUIRES tests that drive write through its mkdir /
// writefile / rename failures, i.e. the negative I/O paths the scout AC2 names.
func TestC321_002_StoreWriteCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "write"); pct < floor {
		t.Errorf("RED: Store.write coverage %.1f%% < %.1f%% — add tests that fail os.MkdirAll "+
			"(read-only parent), os.WriteFile (unwritable temp dir), and os.Rename (target is a "+
			"non-empty dir). Baseline 63.6%%; the marshal-error arm is unreachable so 90%% is the ceiling.", pct, floor)
	}
}

// --- C321_003 (AC3): NewStore coverage >= 90% ---------------------------------
//
// Behavioral + edge axis. NewStore is at 66.7% today: every existing test injects
// a fixed clock, so the `if now == nil { now = time.Now }` default arm is dark.
// Reaching 90% (really 100% for this 3-statement constructor) REQUIRES a
// NewStore(root, nil) call asserting the store still works — the nil-clock
// default the scout AC3 names.
func TestC321_003_NewStoreCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "NewStore"); pct < floor {
		t.Errorf("RED: NewStore coverage %.1f%% < %.1f%% — add a test calling NewStore(root, nil) so "+
			"the `now = time.Now` default arm runs. Baseline 66.7%% (only the injected-clock arm).", pct, floor)
	}
}

// --- C321_004 (AC4): Store.Load coverage >= 90% -------------------------------
//
// Behavioral + edge axis. Load is at 76.9% today: the existing corrupt-JSON test
// hits one degradation arm, but the read-error WARN path (s.path is a directory,
// so os.ReadFile returns a non-IsNotExist error) and the valid-JSON-but-nil-
// `benches` arm are dark. Reaching 90% REQUIRES tests that feed Load an
// unreadable path and a typed-but-bench-less JSON document — the degradation
// arms the scout AC4 names.
func TestC321_004_StoreLoadCoverageFloor(t *testing.T) {
	const floor = 90.0
	out := coverFuncOutput(t)
	if pct := funcCoverage(t, out, "Load"); pct < floor {
		t.Errorf("RED: Store.Load coverage %.1f%% < %.1f%% — add tests for the read-error WARN path "+
			"(path is a directory) and the valid-JSON-but-nil-benches arm. Baseline 76.9%%.", pct, floor)
	}
}

// --- C321_005 (AC5): the existing clihealth contract suite stays green --------
//
// Regression axis. Runs the pre-existing TestStore* / TestCooldown* / TestBench*
// / TestNewBenchEntry / TestParseResetHint* contract suite (bench round-trip,
// lazy expiry, corrupt-degrades, clear, cooldown doubling, reset-hint parsing).
// Pre-existing GREEN today — Builder must KEEP it green while adding the new
// error-path tests. Uses a distinct -run command verb from the coverage gate for
// lexical diversity.
func TestC321_005_ExistingContractSuiteGreen(t *testing.T) {
	combined, code := runGoTest(t, "-run", "TestStore|TestCooldown|TestBench|TestNewBenchEntry|TestParseResetHint",
		"-count=1", clihealthPkg)
	if code != 0 {
		t.Errorf("RED: the existing clihealth contract suite fails (exit=%d) — the new I/O error-path "+
			"tests must not break the bench/cooldown/parse contract:\n%s", code, tail(combined, 30))
	}
}

// --- C321_006 (AC6): write on a read-only root returns a non-nil error --------
//
// Behavioral negative axis (adversarial-testing SKILL §6). Drives the real Store
// through its public Bench entry point against a read-only project root: write's
// os.MkdirAll(<root>/.evolve) cannot create the directory, so write — and thus
// Bench — must surface a non-nil error rather than silently swallowing it (the
// package contract: a write failure is reported, only Load degrades). This
// CALLS the system under test (no source-grep) and a happy-path-only Store could
// not satisfy it.
//
// NOTE: this exercises behavior that already exists, so it is pre-existing GREEN;
// it anchors AC6 as an explicit behavioral contract while the C321_002 write
// coverage floor is the gate that forces Builder to TEST this path from inside
// the clihealth package (this external predicate does not count toward the
// clihealth coverage number).
func TestC321_006_BenchOnReadOnlyRootReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root bypasses directory permissions; read-only-dir negative test is meaningless")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o500); err != nil { // r-x------ : owner cannot create children
		t.Fatalf("chmod read-only root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) }) // restore so t.TempDir cleanup can remove it

	s := clihealth.NewStore(root, nil)
	err := s.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit"})
	if err == nil {
		t.Errorf("RED: Bench on a read-only root must return a non-nil error (write must not " +
			"swallow the os.MkdirAll failure)")
	}
}
