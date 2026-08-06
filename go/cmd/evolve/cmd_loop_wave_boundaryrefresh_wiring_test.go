package main

// cmd_loop_wave_boundaryrefresh_wiring_test.go — cycle 1325, task
// auto-refresh-binary-at-boundary (fleet_scope for this lane).
//
// PRIOR STATE (verified live in this worktree, not assumed from the inbox
// item): cycle-1314 built the full boundary binary-refresh mechanism —
// maybeRefreshChainBoundary (cmd_loop_chain.go) — with the ahead-check,
// rebuild, provenance-gated re-pin, re-exec, and the re-exec loop breaker,
// all independently unit-tested and hardened across cycle-1320
// (cmd_loop_chain_boundaryrefresh_test.go,
// cmd_loop_chain_boundaryrefresh_hardening_test.go — both GREEN in this
// worktree: TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch
// and TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch PASS).
// That mechanism is wired into ONE of the two batch loops: runLoopChain (the
// `evolve loop --chain` path, cmd_loop_chain.go:538).
//
// THE GAP THIS CYCLE CLOSES: the inbox item's own incident (cycles 1302-1309
// running a stale binary) motivated cycle-1314's fix, but runLoopBatch's OWN
// per-wave/fleet batch loop (cmd_loop.go, the `for i := 0; i < effectiveMax;
// i++` loop used by plain `evolve loop --max-cycles N` / fleet mode WITHOUT
// --chain — runLoop itself is a thin dispatcher that hands off to either
// runLoopChain or runLoopBatch) never calls maybeRefreshChainBoundary at all
// — confirmed by acsassert.CountInGoFunc(cmd_loop.go, "runLoopBatch",
// "maybeRefreshChainBoundary") == 0 in this worktree today. A chained loop
// self-heals at every boundary; a non-chained multi-wave/fleet loop does
// not — the exact class of bug cycle-1314 fixed for one caller and left
// open for the other.
//
// FIX CONTRACT (undefined until the Builder adds it — this file's caller-
// proof predicate fails to find the call today, which IS the RED evidence;
// no new logic is invented, maybeRefreshChainBoundary is REUSED verbatim per
// never_duplicate_centralize_via_design_patterns):
//
//	runLoop's wave/fleet batch loop must call
//	  maybeRefreshChainBoundary(cfg, i+1, stderr)
//	at the same boundary point reloadFleetConfigAtWaveBoundary already
//	occupies (before that iteration's wave/lane dispatch — i.e. never
//	mid-lane, structurally guaranteed by the loop's own sequential shape:
//	dispatch only ever starts AFTER this check returns). A true result means
//	a re-exec is imminent/terminal (mirrors runLoopChain's own handling) —
//	the iteration must stop cleanly without starting that wave's dispatch.
//
// Why a structural (AST) caller-proof here, not a full runLoop() behavioral
// drive: maybeRefreshChainBoundary's OWN behavior (fire path, refuse-while-
// already-attempted path, every failure-mode fallback) is ALREADY proven
// GREEN by cycle-1314/1320's tests above — re-driving that behavior through
// a second, much heavier harness (a real runLoop() invocation needs a live
// git repo, storage, launcher, and CLI-health fakes) would duplicate
// coverage, not add it. What is genuinely unverified is only the ONE-LINE
// wiring gap, which this codebase's own precedent (cycle-968
// TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError,
// cmd_loop_wave_minwidth_wiring_test.go) tests exactly this way: an AST
// caller-proof over the real production function's source, waived per
// acsassert's config-check convention because the *behavior* it gates is
// pinned elsewhere.
//
// acs-predicate: config-check — caller-existence is an inherent source-
// structure check; maybeRefreshChainBoundary's behavior is already pinned by
// cmd_loop_chain_boundaryrefresh_test.go / _hardening_test.go (both GREEN).
import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary is the wiring
// proof: runLoop's own per-wave/fleet batch loop must call
// maybeRefreshChainBoundary, not just runLoopChain's. A regression here
// (the call site never added, or later deleted in an unrelated refactor)
// silently reopens the cycles-1302-1309 stale-binary class for every
// non-chained multi-wave/fleet run.
func TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary(t *testing.T) {
	// runLoop itself is a thin dispatcher (parse args -> runLoopChain XOR
	// runLoopBatch); the actual `for i := 0; i < effectiveMax; i++`
	// wave/fleet batch loop this test's own doc comment describes lives in
	// runLoopBatch (cmd_loop.go), so that is the function the call site must
	// land in.
	n, err := acsassert.CountInGoFunc("cmd_loop.go", "runLoopBatch", "maybeRefreshChainBoundary")
	if err != nil {
		t.Fatalf("CountInGoFunc(runLoopBatch, maybeRefreshChainBoundary): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopBatch does not call maybeRefreshChainBoundary (count=%d); the wave/fleet batch loop can still run for hours on a stale binary after a chain-less `evolve loop --max-cycles N` — only runLoopChain gets the cycle-1314 self-heal today", n)
	}
}

// TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers is the negative/
// anti-gaming companion: the wiring must land in runLoop's OWN loop body, not
// merely somewhere inside a dispatch helper it calls (dispatchIteration,
// forceOneLaneDispatch, minWidthRepair) — those run PER-LANE, so a call
// buried there would re-check staleness mid-dispatch instead of at the clean
// boundary the fire/refuse contract requires (never mid-lane). This does not
// re-litigate maybeRefreshChainBoundary's own internal fire/refuse behavior
// (already GREEN elsewhere) — it only pins WHERE the new call must NOT be.
func TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers(t *testing.T) {
	for _, fn := range []string{"dispatchIteration", "forceOneLaneDispatch", "minWidthRepair"} {
		n, err := acsassert.CountInGoFunc("cmd_loop_wave.go", fn, "maybeRefreshChainBoundary")
		if err != nil {
			t.Fatalf("CountInGoFunc(%s, maybeRefreshChainBoundary): %v", fn, err)
		}
		if n > 0 {
			t.Errorf("%s calls maybeRefreshChainBoundary (count=%d) — the refresh belongs at runLoop's own boundary point, not inside a per-lane dispatch helper (would re-check staleness mid-lane)", fn, n)
		}
	}
}
