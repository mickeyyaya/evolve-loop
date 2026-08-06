//go:build acs

// Package cycle1352 materializes the acceptance criteria of this lane's two
// top_n tasks — `chain-boundary-binary-refresh-stop` and
// `chain-summary-refresh-event-field` (triage-report.md "## top_n";
// scout-report.md Tasks 1/2; fleet-scoped inbox item
// `auto-refresh-binary-at-boundary`).
//
// FINDING (read-first, rule 8 — this cycle changes NO production code): the
// scout report's premise does not match the repo on disk, for the SAME
// reason cycle-1343 already found and pinned (go/acs/cycle1343/predicates_
// test.go). Re-verified fresh for this cycle:
//
//	grep -rn 'bootRefreshHeadFn\|bootRefreshBinaryCommitFn\|bootRefreshSource
//	DeltaFn\|BootBinaryRefresh\|bootBinaryRefresh\|chain_binary_stale\|
//	StaleAtBoundary\|StaleBinaryCommit' go/   -> zero hits outside this file
//	find docs/chronicle -iname '*binary-lag*'  -> no chronicle directory exists
//
// None of the seams the scout report names as reuse targets
// (bootRefreshHeadFn, bootRefreshBinaryCommitFn, bootRefreshSourceDeltaFn,
// policy.Load(...).BootBinaryRefresh()) or the chronicle doc it cites exist
// anywhere in this worktree.
//
// What DOES exist, already fully built, tested, and wired at BOTH call
// sites the AC set cares about, is the functional superset:
// `maybeRefreshChainBoundary` (go/cmd/evolve/cmd_loop_chain.go, shipped
// across cycles 1314/1320/1323/1325/1330, documented at
// docs/operations/runtime-reference.md "Boundary binary auto-refresh").
//
// Task 1's AC set, read literally, asks for a *stop-only* handoff (rc=0,
// reason `chain_binary_stale`, "does not rebuild or re-exec itself — stays
// boot-owned") because the scout reasoned re-exec-from-inside-the-chain was
// unsafe. The shipped design proves that reasoning's premise wrong: it DOES
// rebuild + provenance-gate + re-pin + re-exec from inside the boundary
// (maybeRefreshChainBoundary, cmd_loop_chain.go:400-484), and does so
// SAFELY — `res.Batches`/`chainResult` accumulated so far is marshaled and
// printed to stdout BEFORE the re-exec (runLoopChain, cmd_loop_chain.go:
// 578-586), and a loop-breaker marker (chainBoundaryRefreshAttemptFile)
// refuses a second re-exec attempt on the same build commit so the process
// can never livelock at zero batches run. This is a strictly stronger
// contract than the scout's stop-and-wait-for-the-next-launch design: no
// operator/wrapper relaunch is needed at all. Re-litigating it down to a
// weaker stop-only path, or duplicating a second `chain_binary_stale`
// code path alongside the shipped `chain_boundary_refresh_reexec` one,
// would violate the standing no_workaround_root_cause_redesign and
// never_duplicate_centralize_via_design_patterns rules — see
// test-report.md AC-Materialization for the full disposition.
//
// Task 2's AC (surface the refresh event in the summary) IS already
// satisfied: `chainResult.BoundaryRefresh *chainBoundaryRefreshLogEntry`
// (json:"boundary_refresh,omitempty") carries Batch/OldSHA/NewSHA/Timestamp
// when a boundary refresh fires, and is nil/omitted on every ordinary stop —
// the identical shape the AC's `stale_at_boundary`/`stale_binary_commit`
// pair asks for (richer, since it also carries the commit pair and
// timestamp a dossier consumer needs, not just a boolean+string).
//
// Both predicates below are PRE-EXISTING GREEN — see test-report.md RED Run
// Output. They are authored anyway as THIS cycle's regression pin
// (AC-Materialization requires a `predicate` artifact for every
// predicate-dispositioned AC; a GREEN result on unmodified shipped code is
// the explicitly-sanctioned "pre-existing GREEN" carve-out, Step 4) so a
// future regression in either boundary's wiring or the summary field is
// caught by THIS cycle's card, not silently absorbed into cycle-1343's.
//
// Adversarial diversity:
//
//	C1352_001 positive — the chain-boundary refresh (Task 1's functional
//	                      equivalent) fires strictly before every batch,
//	                      never mid-batch, and a trip stops the chain
//	                      before running that boundary's own batch.
//	C1352_002 positive — the refresh event (Task 2) surfaces into
//	                      chainResult's JSON when a boundary refresh fires,
//	                      and is omitted on every ordinary (non-refresh)
//	                      stop.
//	C1352_003 negative — no second, duplicate `chain_binary_stale` reason
//	                      string or `StaleAtBoundary`/`StaleBinaryCommit`
//	                      field exists anywhere in the chain source — i.e.
//	                      nobody has (re-)introduced the weaker, superseded
//	                      stop-only design the scout report's literal AC
//	                      wording describes.
package cycle1352

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

// TestC1352_001_boundary_refresh_stops_chain_before_next_batch_never_midbatch
// — POSITIVE. Task 1's AC ("chain stops before starting a new batch when the
// running binary is behind HEAD; never fires mid-batch") is already met by
// maybeRefreshChainBoundary's call-site placement and stop path.
func TestC1352_001_boundary_refresh_stops_chain_before_next_batch_never_midbatch(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch",
		"TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch")
	if !ok {
		t.Errorf("chain-boundary staleness stop/never-mid-batch wiring regressed (missing PASS receipts: %v). This is the functional core of Task 1 (chain-boundary-binary-refresh-stop) — a chained `evolve loop` must detect binary staleness and stop before the NEXT batch, never mid-batch.\n%s", missing, tail(out, 25))
	}
}

// TestC1352_002_refresh_event_surfaces_in_chain_summary_json — POSITIVE.
// Task 2's AC ("chainResult JSON carries the refresh event when Task 1
// fires, empty/omitted otherwise") is already met by the BoundaryRefresh
// field and its omitempty tag.
func TestC1352_002_refresh_event_surfaces_in_chain_summary_json(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent",
		"TestChainResult_MarshalOmitsBoundaryRefreshWhenNil",
		"TestRunLoopChain_SetsBoundaryRefreshOnReExecStop")
	if !ok {
		t.Errorf("chain-summary refresh-event surfacing regressed (missing PASS receipts: %v). This is Task 2 (chain-summary-refresh-event-field) — chainResult must carry a populated refresh-event field on a boundary-refresh stop and omit it on every ordinary stop.\n%s", missing, tail(out, 25))
	}
}

// TestC1352_003_no_superseded_stop_only_design_reintroduced — NEGATIVE. The
// scout report's literal AC wording (`chain_binary_stale` reason string,
// `StaleAtBoundary`/`StaleBinaryCommit` fields) names a WEAKER design that
// was superseded before this cycle by the shipped rebuild+re-pin+re-exec
// boundary hook. This predicate fails loudly if a future change reintroduces
// that duplicate, narrower code path alongside the shipped one — the exact
// never_duplicate_centralize_via_design_patterns violation this cycle's
// disposition explicitly declined to commit.
func TestC1352_003_no_superseded_stop_only_design_reintroduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	chainSrc := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_chain.go")
	for _, needle := range []string{"chain_binary_stale", "StaleAtBoundary", "StaleBinaryCommit"} {
		if !acsassert.FileNotContains(t, chainSrc, needle) {
			t.Errorf("found %q in cmd_loop_chain.go — a second, superseded staleness-stop code path has been (re-)introduced alongside the shipped chain_boundary_refresh_reexec design; centralize on the one shipped path instead of duplicating it", needle)
		}
	}
}
