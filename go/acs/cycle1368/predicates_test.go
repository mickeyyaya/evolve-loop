//go:build acs

// Package cycle1368 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `auto-refresh-binary-at-boundary`
// (scout-report.md Task 1 — "Wire bootBinaryRefreshFn into the
// wave-boundary block of runLoopBatch").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout's premise names `bootBinaryRefreshFn` /
// `go/cmd/evolve/cmd_loop_boot_refresh.go` as the seam to wire. Neither
// exists anywhere in this worktree's checked-out source:
//
//	grep -rn 'bootBinaryRefreshFn|BootBinaryRefresh' go/   -> zero hits
//	find go -iname 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main is 71 commits behind current
// main (`git rev-list --count HEAD..origin/main` = 71); the boot-time
// rebuild+re-exec self-heal the scout report describes is a main-line
// feature this worktree's snapshot predates — inventing a predicate against
// it would mean inventing an API (rule 8 forbids it).
//
// What DOES exist, already fully built, wired, and GREEN, is the
// FUNCTIONAL EQUIVALENT the AC set actually asks for:
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go), called at
//   - the chain boundary, inside runLoopChain's per-batch loop
//     (cmd_loop_chain.go:626), AND
//   - the plain wave/fleet boundary inside runLoopBatch's per-iteration
//     loop (cmd_loop.go:552) — the EXACT call site scout's Task 1 asks to
//     add, already present, right where scout said to put it (next to
//     `reloadFleetConfigAtWaveBoundary` / `syncMainFromOriginAtWaveBoundary`,
//     cmd_loop.go:535-559).
//
// This is the SAME finding cycles 1343, 1352, and 1356 already made and
// pinned for this same recurring inbox item (go/acs/cycle1343,
// go/acs/cycle1352, go/acs/cycle1356 — all present on this worktree's HEAD).
// The item keeps re-entering fleet_scope (queue-hygiene residual, tracked in
// operator memory as "queued at priority 0.55") faster than the consuming
// commit lands; re-verified fresh for cycle 1368:
//
//	go build ./...                                          -> clean
//	go test -run BoundaryRefresh ./cmd/evolve/...            -> all PASS
//
// No new production code is warranted. This cycle's predicates
// REGRESSION-LOCK the already-shipped contract for THIS cycle's audit gate
// (ACS predicates are cycle-scoped, never replayed by a later gate —
// test-report.md Step 6b) via named `go test -run` subprocess assertions,
// the established idiom for `package main` coverage from `go/acs`
// (cycle-1352/1356 precedent).
//
// Adversarial diversity:
//
//	C1368_001 positive — the wave-boundary call site scout's Task 1 literally
//	                      asks for exists and fires before dispatch, never
//	                      mid-lane (AC1/AC2).
//	C1368_002 positive — the chain-boundary call site (the other half of the
//	                      same seam) fires strictly before every batch, never
//	                      mid-batch (AC1, chain-mode coverage).
//	C1368_003 positive — the fail-open contract: any staleness/rebuild check
//	                      failure degrades to refreshed=false, batch/chain
//	                      keeps running on the old binary (AC3).
//	C1368_004 positive — the refresh is ledgered under a distinguishable
//	                      "boundary-refresh" authorization class (AC4).
//	C1368_005 negative — no second, duplicate stop-only staleness code path
//	                      has been (re-)introduced alongside the shipped
//	                      rebuild+re-pin+re-exec design.
package cycle1368

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// mainPkg is the single named package under test. Narrowed further by -run on
// every invocation; never a /... sweep (flaky-predicate-shape rule 1).
const mainPkg = "./cmd/evolve"

// goDir is the CYCLE's go module root — acsassert.RepoRoot resolves the
// worktree, so predicates read this lane's source, not main's.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./cmd/evolve` and reports whether EVERY named test both RAN and PASSED.
// code<0 is a genuine launch failure (fatal); a zero exit with a missing
// `--- PASS: <name>` receipt means the test was deleted/skipped, reported as
// a miss rather than a pass (cycle-352 broken-predicate idiom).
func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", mainPkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", mainPkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

// tail returns the last n lines so verdict diagnostics stay readable.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestC1368_001_wave_boundary_call_site_scout_asked_for_already_wired —
// POSITIVE. AC1/AC2 (Task 1, literal wording): the wave/fleet-boundary call
// site inside runLoopBatch — exactly where scout's Task 1 asks it to be
// added, next to reloadFleetConfigAtWaveBoundary/
// syncMainFromOriginAtWaveBoundary — already exists, fires once per wave
// before dispatch, and is never invoked from inside a dispatch helper (i.e.
// never mid-lane).
func TestC1368_001_wave_boundary_call_site_scout_asked_for_already_wired(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("wave/fleet-boundary refresh wiring regressed (missing PASS receipts: %v). Scout's Task 1 literal ask — a wave-boundary call to the binary-refresh seam — already exists at this call site; a regression here reopens the exact gap scout described.\n%s", missing, tail(out, 25))
	}
}

// TestC1368_002_chain_boundary_call_site_fires_before_every_batch_never_midbatch
// — POSITIVE. AC1 (chain-mode coverage, the other half of the same seam):
// maybeRefreshChainBoundary's ahead-check runs immediately before EVERY
// chain batch (strict [ahead-check, batch] pairing, never mid-batch), and a
// boundary that trips the refresh stops the chain having run ZERO batches of
// its own.
func TestC1368_002_chain_boundary_call_site_fires_before_every_batch_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger")
	if !ok {
		t.Errorf("chain-boundary self-heal / never-mid-batch wiring regressed (missing PASS receipts: %v).\n%s", missing, tail(out, 25))
	}
}

// TestC1368_003_staleness_check_failure_fails_open — POSITIVE. AC3: any
// staleness/rebuild/repin check failure degrades to refreshed=false; the
// batch/chain keeps running on the old binary rather than halting.
func TestC1368_003_staleness_check_failure_fails_open(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh",
		"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh")
	if !ok {
		t.Errorf("boundary-refresh fail-open contract regressed (missing PASS receipts: %v). AC3 requires every staleness/rebuild/repin failure to degrade to refreshed=false, never a halt.\n%s", missing, tail(out, 25))
	}
}

// TestC1368_004_authorization_class_is_distinguishable_from_boot —
// POSITIVE. AC4: a successful boundary refresh is ledgered under a
// distinguishable "boundary-refresh" authorization class, separate from the
// boot-time class.
func TestC1368_004_authorization_class_is_distinguishable_from_boot(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
		"TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent")
	if !ok {
		t.Errorf("boundary-refresh auditable authorization-class wiring regressed (missing PASS receipts: %v). AC4 requires the boundary class to stay distinguishable from the boot-time class in the audit trail.\n%s", missing, tail(out, 25))
	}
}

// TestC1368_005_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal wording (and its predecessors', cycles 1343/1352/
// 1356) each pinned that a WEAKER stop-only design must never resurface
// alongside the shipped rebuild+re-pin+re-exec boundary hook. Fails loudly
// if a future change reintroduces that duplicate, narrower code path.
func TestC1368_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}
