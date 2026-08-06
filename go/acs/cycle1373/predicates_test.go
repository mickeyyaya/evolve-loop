//go:build acs

// Package cycle1373 materializes the acceptance criteria of this lane's sole
// fleet-scoped inbox item `auto-refresh-binary-at-boundary`
// (scout-report.md Task 1 — "chain-boundary-binary-refresh"; triage
// top_n — "call bootBinaryRefreshFn from runLoopChain at batch boundaries
// (n>0), reusing existing staleness/fleet-lane-guard machinery").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout's premise (and triage's literal top_n text, carried over verbatim
// from scout) names `go/cmd/evolve/cmd_loop_boot_refresh.go` /
// `bootBinaryRefreshFn` as an existing, callable helper this lane merely
// needs to wire into `runLoopChain`. Neither the file nor the symbol
// exists anywhere in this worktree's checked-out source:
//
//	grep -rn 'func bootBinaryRefreshFn|BootBinaryRefresh' go/   -> zero hits
//	find go -iname 'cmd_loop_boot_refresh*.go'                 -> no file
//
// `git merge-base --is-ancestor 2329f350 HEAD` (the commit that landed
// `cmd_loop_boot_refresh.go` on `main`) reports NOT an ancestor of this
// worktree's HEAD — this lane's branch diverged from `main` at 4dadf62a,
// before that boot-time-only self-heal landed upstream. Inventing a call
// to a function that does not exist on this branch would be rule-8
// API-invention, not a real fix.
//
// What DOES exist, already fully built, wired, and GREEN on THIS
// worktree's HEAD (commit 31790b6d, "salvage snapshot"), is the exact
// chain-boundary self-heal the inbox item's fix text asks for —
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go:475),
// called at:
//   - the chain boundary, inside runLoopChain's per-batch loop
//     (cmd_loop_chain.go:660), strictly after the operator-brake check and
//     strictly before runLoopBatchFn — never mid-batch; AND
//   - the plain wave/fleet boundary inside runLoop's per-iteration loop
//     (cmd_loop.go:552), before any lane/wave dispatch starts that
//     iteration — never mid-lane.
//
// This is the SAME finding cycles 1314/1323/1340/1343/1352/1356/1368/1370
// already made and pinned for this same recurring inbox item (their
// go/acs/cycle<N> predicate files are all present on this worktree's
// history) — the item keeps re-entering fleet_scope (queue-hygiene
// residual, operator memory: "queued at priority 0.55") faster than the
// consuming commit that would remove it from the backlog lands. Re-verified
// fresh for cycle 1373:
//
//	go build ./...                                          -> clean
//	go vet ./cmd/evolve/...                                  -> clean
//	go test ./cmd/evolve/...                                 -> ok (full package, no regression)
//	go test -run BoundaryRefresh ./cmd/evolve/... -v         -> all PASS
//
// No new production code is warranted (Operating Principle 1 — TDD must
// NOT implement production code, and there is nothing missing to
// implement). This cycle's predicates REGRESSION-LOCK the already-shipped
// contract for THIS cycle's audit gate (ACS predicates are cycle-scoped,
// never replayed by a later gate — test-report.md Step 6b) via named
// `go test -run` subprocess assertions against the real production callers
// (House Rule 2 — caller proof), the established idiom for `package main`
// coverage from `go/acs` (cycle-1352/1356/1368/1370 precedent).
//
// Adversarial diversity:
//
//	C1373_001 positive — the wave-boundary call site (cmd_loop.go:552)
//	                      fires before dispatch, never mid-lane (AC1).
//	C1373_002 positive — the chain-boundary call site
//	                      (cmd_loop_chain.go:660) fires strictly before
//	                      every batch, never mid-batch (AC1, AC5's
//	                      chain-mode half).
//	C1373_003 positive — the fail-open + fleet-lane-guard contract: any
//	                      staleness/rebuild check failure, AND a live
//	                      sibling fleet lane, both degrade to
//	                      refreshed=false with no rebuild invoked (AC3,
//	                      AC4 — "refuse mid-batch (lanes in flight)").
//	C1373_004 positive — the re-exec loop breaker: two consecutive
//	                      boundary hits at the same running commit refuse
//	                      the second rebuild attempt (AC5), and a
//	                      successful refresh is ledgered under a
//	                      distinguishable "boundary-refresh" authorization
//	                      class (AC2 — "an auditable ... authorization
//	                      class (NOT silent)").
//	C1373_005 negative — no second, superseded stop-only staleness code
//	                      path has been (re-)introduced alongside the
//	                      shipped rebuild+re-pin+re-exec boundary design.
package cycle1373

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

// TestC1373_001_wave_boundary_call_site_fires_before_dispatch_never_midlane —
// POSITIVE. AC1: the wave/fleet-boundary call site inside runLoop
// (cmd_loop.go:552) fires once per wave before dispatch, and is never
// invoked from inside a dispatch helper (i.e. never mid-lane).
func TestC1373_001_wave_boundary_call_site_fires_before_dispatch_never_midlane(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("wave/fleet-boundary refresh wiring regressed (missing PASS receipts: %v). This call site is the literal AC1 ask — a regression here reopens the gap the inbox item exists to close.\n%s", missing, tail(out, 25))
	}
}

// TestC1373_002_chain_boundary_call_site_fires_before_every_batch_never_midbatch
// — POSITIVE. AC1/AC5 (chain-mode half): maybeRefreshChainBoundary's
// ahead-check runs immediately before EVERY chain batch (strict
// [ahead-check, batch] pairing, never mid-batch), and a boundary that trips
// the refresh stops the chain having run ZERO batches of its own.
func TestC1373_002_chain_boundary_call_site_fires_before_every_batch_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger")
	if !ok {
		t.Errorf("chain-boundary self-heal / never-mid-batch wiring regressed (missing PASS receipts: %v).\n%s", missing, tail(out, 25))
	}
}

// TestC1373_003_failopen_and_fleetlane_guard_refuse_rebuild — POSITIVE.
// AC3/AC4: any staleness/rebuild/ahead-check failure, AND a live sibling
// fleet lane holding a fresh run lease, both degrade to refreshed=false
// with NO rebuild command invoked — the standing rule "NEVER rebuild the
// plane binary mid-batch" (project memory stale_binary_false_fail), and
// the inbox item's own "Refuse mid-batch (lanes in flight)" clause.
func TestC1373_003_failopen_and_fleetlane_guard_refuse_rebuild(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh",
		"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh",
		"TestMaybeRefreshChainBoundary_FleetLaneActiveRefusesRebuild",
		"TestMaybeRefreshChainBoundary_FleetLaneCheckErrorRefusesRebuild")
	if !ok {
		t.Errorf("boundary-refresh fail-open / fleet-lane-guard contract regressed (missing PASS receipts: %v). AC3/AC4 require every staleness/rebuild/fleet-lane check that cannot prove the plane is idle to refuse the rebuild, never halt or clobber a sibling lane.\n%s", missing, tail(out, 25))
	}
}

// TestC1373_004_loop_breaker_refuses_second_attempt_and_ledgers_class —
// POSITIVE. AC5 (loop-breaker half) + AC2: two consecutive boundary hits at
// the same running commit refuse the second rebuild attempt, and a
// successful refresh is ledgered under a "boundary-refresh" authorization
// class distinguishable in the audit trail (the inbox item's own "an
// auditable 'boundary-refresh' authorization class (NOT silent)" clause).
func TestC1373_004_loop_breaker_refuses_second_attempt_and_ledgers_class(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker",
		"TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent")
	if !ok {
		t.Errorf("boundary-refresh loop-breaker / auditable authorization-class wiring regressed (missing PASS receipts: %v). AC5 requires a second same-commit attempt to be refused without a second rebuild; AC2 requires the boundary class to stay distinguishable in the audit trail.\n%s", missing, tail(out, 25))
	}
}

// TestC1373_005_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal wording (and its predecessors', cycles
// 1314/1323/1340/1343/1352/1356/1368/1370) each pinned that a WEAKER
// stop-only design must never resurface alongside the shipped
// rebuild+re-pin+re-exec boundary hook. Fails loudly if a future change
// reintroduces that duplicate, narrower code path.
func TestC1373_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}
