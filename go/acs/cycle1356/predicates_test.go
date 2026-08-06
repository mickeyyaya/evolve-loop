//go:build acs

// Package cycle1356 materializes the acceptance criteria of this lane's fleet
// -scoped inbox item `auto-refresh-binary-at-boundary` (scout-report.md Tasks
// 1/2: `chain-boundary-binary-refresh`, `pin-boundary-repin-branch-residual`).
//
// FINDING (read-first, rule 8 — this cycle changes NO production code): Task
// 1's implementation already exists in this worktree, in full, GREEN — the
// SAME finding cycles 1343 and 1352 already made and pinned for this same
// recurring inbox item (go/acs/cycle1343, go/acs/cycle1352). Re-verified
// fresh for cycle 1356:
//
//	grep -rn 'maybeRefreshChainBoundary|chainBoundaryRefreshLogFile' go/cmd/evolve/cmd_loop_chain.go
//	  -> present, fully wired at runLoopChain's per-batch boundary
//	go test -run BoundaryRefresh ./cmd/evolve/...  -> all PASS
//
// Task 2's residual, however, is NOT covered by any prior cycle
// (`grep -rln 'attemptBootRepin' go/cmd/evolve/*_test.go` -> zero hits before
// this cycle). It also cannot be tested AS LITERALLY SCOPED: the scout
// report's target file `cmd_loop_boot_refresh.go` and seam
// `bootRefreshRepinFn` do not exist anywhere in this worktree's checked-out
// source —
//
//	grep -rn 'bootRefreshRepinFn|BootBinaryRefresh' go/   -> zero hits
//	find go -name 'cmd_loop_boot_refresh*.go'             -> no file
//
// This worktree's merge-base with origin/main (4dadf62a923640c) is 71 commits
// behind current main; the boot-time rebuild+re-exec self-heal the scout
// report describes is a main-line feature this worktree's snapshot predates.
// Writing a predicate against `cmd_loop_boot_refresh.go` here would mean
// inventing an API — rule 8 forbids it.
//
// The residual's INTENT — pin which of two repin call sites either side of a
// re-exec heals, and prove the other is a documented no-op, not an
// accidental race — is preserved and applied to the mechanism this worktree
// actually has: maybeRefreshChainBoundary's pre-exec repin (the "parent"
// branch) versus the re-exec'd child's boot-recovery repin
// (attemptBootRepin, gated by detectShipSHAMismatch — the "child" branch).
// See test-report.md AC-Materialization for the full disposition and the
// one-line reason for this reframing (rule 3, no silent guessing).
//
// Adversarial diversity:
//
//	C1356_001 positive — Task 1's functional core: the chain-boundary refresh
//	                      fires strictly before every batch, never mid-batch,
//	                      and a trip stops the chain before that boundary's
//	                      own batch (AC1/AC2).
//	C1356_002 positive — Task 1's fail-open contract: an ahead-check error (or
//	                      any staleness-check failure) degrades to
//	                      refreshed=false and the chain keeps running on the
//	                      current binary rather than halting (AC3).
//	C1356_003 positive — Task 1's auditable authorization class: a successful
//	                      boundary refresh is ledgered under the
//	                      distinguishable "boundary-refresh" class, separate
//	                      from the boot-time class (AC4).
//	C1356_004 positive — Task 2 (reframed): the parent branch
//	                      (maybeRefreshChainBoundary) performs the heal
//	                      before re-exec; the child branch's own repin
//	                      (attemptBootRepin) is a documented no-op both via
//	                      the upstream mismatch gate and directly.
//	C1356_005 negative — no second, duplicate stop-only staleness code path
//	                      (`chain_binary_stale`/`StaleAtBoundary`) has been
//	                      (re-)introduced alongside the shipped
//	                      rebuild+re-pin+re-exec design.
package cycle1356

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

// TestC1356_001_boundary_refresh_fires_only_at_boundary_never_midbatch —
// POSITIVE. Task 1 AC1/AC2: chain running maxBatches>=2 self-heals on a
// go/-touching drift discovered at a batch boundary; never mid-batch.
func TestC1356_001_boundary_refresh_fires_only_at_boundary_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch",
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
		"TestRunLoop_BoundaryRefreshNeverCalledInsideDispatchHelpers")
	if !ok {
		t.Errorf("chain-boundary self-heal / never-mid-batch wiring regressed (missing PASS receipts: %v). This is Task 1's functional core (chain-boundary-binary-refresh).\n%s", missing, tail(out, 25))
	}
}

// TestC1356_002_staleness_check_failure_fails_open — POSITIVE. Task 1 AC3:
// a staleness-check error degrades to refreshed=false; the chain continues
// on the old binary rather than halting.
func TestC1356_002_staleness_check_failure_fails_open(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh",
		"TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip",
		"TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh")
	if !ok {
		t.Errorf("boundary-refresh fail-open contract regressed (missing PASS receipts: %v). AC3 requires every staleness/rebuild/repin failure to degrade to refreshed=false, never a halt.\n%s", missing, tail(out, 25))
	}
}

// TestC1356_003_authorization_class_is_distinguishable_from_boot — POSITIVE.
// Task 1 AC4: a successful boundary refresh is ledgered under a
// distinguishable "boundary-refresh" authorization class.
func TestC1356_003_authorization_class_is_distinguishable_from_boot(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger",
		"TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent")
	if !ok {
		t.Errorf("boundary-refresh auditable authorization-class wiring regressed (missing PASS receipts: %v). AC4 requires the boundary class to stay distinguishable from the boot-time class in the audit trail.\n%s", missing, tail(out, 25))
	}
}

// TestC1356_004_parent_branch_heals_child_branch_is_documented_noop —
// POSITIVE. Task 2 (reframed, see package doc): the parent boundary-refresh
// repin performs the heal before re-exec; the child boot-recovery repin is a
// documented no-op, both via the upstream mismatch gate and directly.
func TestC1356_004_parent_branch_heals_child_branch_is_documented_noop(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestMaybeRefreshChainBoundary_PrePinsBeforeReExecSoChildBootRepinIsNoOp",
		"TestAttemptBootRepin_NoOpWhenPinAlreadyMatchesOnDiskBinary")
	if !ok {
		t.Errorf("parent-heals/child-no-ops repin-branch split regressed (missing PASS receipts: %v). This is Task 2 (pin-boundary-repin-branch-residual), reframed onto the boundary-refresh mechanism this worktree actually carries.\n%s", missing, tail(out, 25))
	}
}

// TestC1356_005_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal wording names a WEAKER stop-only design that was
// superseded before this cycle by the shipped rebuild+re-pin+re-exec
// boundary hook (cycles 1343/1352 already pinned this). Fails loudly if a
// future change reintroduces that duplicate, narrower code path.
func TestC1356_005_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}
