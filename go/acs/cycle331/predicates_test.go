//go:build acs

// Package cycle331 materializes the cycle-331 acceptance criteria for the TWO
// committed top_n tasks (triage-report.md "## top_n"), both lifting coverage of
// the OS write-error branches in internal/adapters/ledger (84.29% baseline):
//
//	ledger-seal-write-error-branches — exercise the dark write-error branches in
//	    seal.go: writeSegment's os.CreateTemp failure (read-only parent dir;
//	    62.5% baseline) and rewriteLive's CreateTemp failure (read-only ledger
//	    dir; 65.2% baseline). The named tests are TestWriteSegment_MkdirError,
//	    TestWriteSegment_CreateTempError, TestRewriteLive_CreateTempError.
//
//	ledger-anchor-write-error-branch — exercise the dark write-error branches in
//	    anchor.go: Anchor's gatherAllLines propagation, CreateTemp failure, and
//	    final-rename failure (65.6% baseline), plus loadAnchorSHA's corrupt-JSON
//	    degradation to "" (85.7% baseline). Named tests TestAnchor_CreateTempError,
//	    TestAnchor_RenameError, TestAnchor_GatherError, TestLoadAnchorSHA_CorruptJSON.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) — there is NO load-bearing
// source-grep:
//
//   - The named-test gates RUN the real ledger suite in a subprocess filtered to
//     the committed test functions and require both a clean exit AND each
//     `--- PASS:` line. A magic string in a source file cannot make a named test
//     appear and PASS; an EMPTY repo (no such tests) produces "no tests to run"
//     and zero PASS lines, so the gates are RED today and only Builder's real
//     error-branch tests turn them GREEN. They encode the scout verifiableBy
//     commands verbatim.
//   - The coverage gates RUN the real package suite under -coverprofile and
//     assert on the measured `go tool cover -func` percentages. A magic string
//     cannot move a coverage number — only Builder's new tests can — so they are
//     anti-no-op by construction; an EMPTY repo yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, folding the no-regression axis into every coverage gate; a dedicated
//     -race gate adds the data-race axis with a distinct command verb.
//   - The per-function anchors (writeSegment / Anchor / loadAnchorSHA) pin the %
//     movement to the TARGET dark branches, not to incidental coverage elsewhere.
//
// ⚠ FLOOR-CALIBRATION NOTE (Core Rule 3 — surfaced conflict; cycle-280 lesson —
// never pin an UNREACHABLE floor or it starves the committed work):
// the scout's "Both: package coverage >= 87%" AC is NOT reachable with the
// chmod fault-injection technique the scout itself specified. Line-level
// analysis of the cover profile shows the only NEW statements the planned tests
// can cover are 7 (writeSegment CreateTemp +1, gatherAllLines readSegment +1,
// loadAnchorSHA unmarshal +1, Anchor gather/CreateTemp/rename +4) → 354+7 =
// 361/420 = 85.95%. The gzip-writer / tmp.Write / tmp.Sync / tmp.Close / final
// os.Rename error arms in writeSegment, rewriteLive, and Anchor are UNREACHABLE
// by filesystem tricks (a freshly-created regular file does not fail
// write/fsync/close portably — the SAME finding the cycle-322 modelcatalog eval
// documents). Reaching 87% would need +12 covered statements, i.e. covering
// those fd-level arms, which requires a production write-seam the scout did not
// scope. So the package gate (C331_006) is pinned at the ACHIEVABLE >= 85.2%
// (+4 over the 84.29% baseline, 3-statement margin under the ~86.0% realistic
// max), NOT 87%. test-report.md dispositions the 87% AC as falsified→adjusted.
//
// AC map (1:1 with the committed-task acceptance criteria):
//
//	ledger-seal-write-error-branches
//	  AC-S1 the three seal write-error tests exist & PASS  → C331_001 (named-test gate)
//	  AC-S2 writeSegment CreateTemp branch covered         → C331_002 (writeSegment >= 66%)
//	ledger-anchor-write-error-branch
//	  AC-A1 the four anchor write-error tests exist & PASS → C331_003 (named-test gate)
//	  AC-A2 Anchor gather/CreateTemp/rename branches covered→ C331_004 (Anchor >= 75%)
//	  AC-A3 loadAnchorSHA corrupt-JSON branch covered      → C331_005 (loadAnchorSHA >= 95%)
//	both
//	  AC-B1 package coverage lifts to the achievable floor → C331_006 (package >= 85.2%, adjusted from 87%)
//	  AC-B2 ledger suite stays green under -race           → C331_007
//
// Floor binding (R9.3): internal/adapters/ledger is the SOLE package both
// committed top_n tasks target, so every floor/gate binds committed work. No
// triage-DEFERRED item (looppreflight saveVersionCache, interaction PromoteRule)
// gets a predicate here — a floor on a deferred task would starve the committed
// ones (cycle-280 lesson).
package cycle331

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

const ledgerPkg = "./internal/adapters/ledger/"

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

// coverFuncOutput runs the REAL package suite under -coverprofile and returns
// the `go tool cover -func` report. Fatals (RED) if the suite does not compile
// or a test FAILs — a coverage number is only meaningful over a green suite, so
// this folds in the no-regression axis.
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

// totalCoverage parses the trailing `total:\t(statements)\t<pct>%` row.
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

// requirePassLines runs the ledger suite filtered to the given -run regex with
// -v and requires a clean exit PLUS a `--- PASS: <name>` line for every name.
// This is the anti-no-op binding for a coverage task whose deliverable is named
// test functions: `go test -run` over a NON-matching pattern exits 0 with
// "no tests to run" and ZERO PASS lines (verified RED today), so requiring the
// explicit PASS lines makes the gate fail until Builder's real tests exist and
// pass. A renamed/removed/failing test drops its PASS line and re-reddens the
// gate; a source-grep or empty repo can never satisfy it.
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

// ================ ledger-seal-write-error-branches predicates ================

// --- C331_001 (AC-S1): the three seal write-error tests exist & PASS ---------
//
// Behavioral. Encodes the scout verifiableBy verbatim:
//
//	go test ./internal/adapters/ledger/... -run \
//	  "TestWriteSegment_MkdirError|TestWriteSegment_CreateTempError|TestRewriteLive_CreateTempError"
//
// RED today: none of the three tests exist, so the -run pattern matches nothing
// ("no tests to run", zero PASS lines). GREEN only when Builder adds tests that
// drive os.MkdirAll / os.CreateTemp to error (a regular file blocking dir
// creation; a read-only parent dir) and assert a non-nil error.
func TestC331_001_SealWriteErrorTestsPass(t *testing.T) {
	requirePassLines(t,
		"TestWriteSegment_MkdirError|TestWriteSegment_CreateTempError|TestRewriteLive_CreateTempError",
		[]string{
			"TestWriteSegment_MkdirError",
			"TestWriteSegment_CreateTempError",
			"TestRewriteLive_CreateTempError",
		})
}

// --- C331_002 (AC-S2): writeSegment coverage >= 66% --------------------------
//
// Behavioral + branch anchor. writeSegment is 62.5% today (15/24 statements):
// the os.CreateTemp error arm (`return fmt.Errorf("ledger seal: tmp: %w", ...)`)
// is dark (the MkdirAll-error and final-rename-error arms are ALREADY covered).
// Reaching 66% REQUIRES a test that makes CreateTemp fail (read-only parent
// dir) — +1 statement → 16/24 = 66.67%. The gzip-write/close, sync, and
// tmp.Close error arms are UNREACHABLE by filesystem tricks (cycle-322 finding),
// so ~66.7% is writeSegment's practical ceiling — the floor is 66, not 75
// (the scout hypothesis of 75%+ is falsified; see floor-calibration note).
func TestC331_002_WriteSegmentCreateTempCovered(t *testing.T) {
	const floor = 66.0
	out := coverFuncOutput(t, ledgerPkg)
	if pct := funcCoverage(t, out, "writeSegment"); pct < floor {
		t.Errorf("RED: ledger.writeSegment coverage %.1f%% < %.1f%% — add a test driving os.CreateTemp "+
			"to error (read-only parent dir). Baseline 62.5%%; the gzip/sync/close arms are unreachable "+
			"by fs tricks so ~66.7%% is the ceiling.", pct, floor)
	}
}

// ================ ledger-anchor-write-error-branch predicates ================

// --- C331_003 (AC-A1): the four anchor write-error tests exist & PASS --------
//
// Behavioral. Encodes the scout verifiableBy verbatim:
//
//	go test ./internal/adapters/ledger/... -run \
//	  "TestAnchor_CreateTempError|TestAnchor_RenameError|TestAnchor_GatherError|TestLoadAnchorSHA_CorruptJSON"
//
// RED today: none of the four tests exist. GREEN only when Builder adds tests
// that (1) make Anchor's os.CreateTemp fail (read-only dir), (2) make the final
// rename fail, (3) make gatherAllLines propagate a read error (a non-gzip file
// in ledger-segments/), and (4) feed loadAnchorSHA corrupt JSON and assert it
// degrades to "" without panicking.
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

// --- C331_004 (AC-A2): Anchor coverage >= 75% --------------------------------
//
// Behavioral + branch anchor. Anchor is 65.6% today (21/32 statements). Three
// dark arms are reachable: the gatherAllLines error propagation, the
// os.CreateTemp error, and the final os.Rename error (+4 statements → 25/32 =
// 78.1%). Reaching 75% REQUIRES tests that drive those. The marshal-error arm
// (unreachable — ledgerAnchor always marshals) and the post-open f.Write/f.Close
// arms (unreachable by fs tricks) cap Anchor at ~81%, so the floor is 75, not 80.
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

// --- C331_005 (AC-A3): loadAnchorSHA coverage >= 95% -------------------------
//
// Behavioral + negative anchor. loadAnchorSHA is 85.7% today (6/7 statements):
// the json.Unmarshal error arm (`return ""` on corrupt anchor JSON) is dark.
// Reaching 95% REQUIRES a test that writes a non-JSON ledger-anchor.json and
// asserts loadAnchorSHA degrades to "" (full-strict) without panicking — +1
// statement → 7/7 = 100%. A magic string cannot move this; only the corrupt-JSON
// test can.
func TestC331_005_LoadAnchorSHACorruptJSONCovered(t *testing.T) {
	const floor = 95.0
	out := coverFuncOutput(t, ledgerPkg)
	if pct := funcCoverage(t, out, "loadAnchorSHA"); pct < floor {
		t.Errorf("RED: ledger.loadAnchorSHA coverage %.1f%% < %.1f%% — add a test feeding corrupt "+
			"(non-JSON) anchor data and asserting it degrades to \"\". Baseline 85.7%%.", pct, floor)
	}
}

// ============================ both-task predicates ===========================

// --- C331_006 (AC-B1): package coverage >= 85.2% (ADJUSTED from scout's 87%) -
//
// Behavioral + headline anti-no-op gate. internal/adapters/ledger is 84.29%
// today (354/420 statements). The scout's "Both: >= 87%" AC is UNREACHABLE with
// the chmod technique it specified — see the package floor-calibration note. The
// only NEW statements the planned tests can cover are 7 (→ 361/420 = 85.95%);
// 87% would need +12, i.e. the fd-level gzip/Write/Sync/Close error arms that no
// filesystem trick can inject. So this gate is pinned at the ACHIEVABLE >= 85.2%
// (+4 over baseline, ~3-statement margin under the realistic max). A no-op
// implementation cannot move package coverage at all, so the gate is still
// strongly anti-no-op; test-report.md records the 87%→85.2% disposition.
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

// --- C331_007 (AC-B2): the ledger suite stays green under -race --------------
//
// Regression + data-race axis. Runs the full ledger suite under -race and
// requires a clean exit. Pre-existing GREEN today; Builder must KEEP it green
// while adding the seal + anchor error-branch tests. Distinct command verb
// (-race, no coverprofile) from the coverage gates for lexical diversity.
func TestC331_007_LedgerSuiteGreenRace(t *testing.T) {
	combined, code := runGoTest(t, "-race", "-count=1", ledgerPkg)
	if code != 0 {
		t.Errorf("RED: internal/adapters/ledger suite fails under -race (exit=%d) — the new "+
			"write-error tests must not break the seal/anchor contract:\n%s", code, tail(combined, 30))
	}
}
