//go:build acs

// Package cycle1098 materialises the cycle-1098 acceptance criteria for the two
// triage-committed top_n tasks of the `chain-policy-flag` lane (scout-report.md /
// triage-report.md). The lane's headline feature (batch chaining) LANDED at
// cycle 1075; this cycle fixes its two residual defects:
//
//   - chain-min-one-batch — `--until-inbox-empty` against an already-drained
//     inbox returns rc=0 having run ZERO cycles, silently weaker than the
//     pre-chain contract where `evolve loop` always ran one batch.
//   - chain-inbox-pending-validity — inboxPendingCount counts every root-level
//     `*.json` with no shape validation, so one malformed file pins pending>0
//     permanently and the chain burns batches to max_batches on a FALSE signal,
//     silently skipping nothing and hiding a real item lost to a typo.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1064 precedent).
// The subjects live in `package main` (go/cmd/evolve), which cannot be imported,
// so each predicate shells `go test -run` over the RED contract tests authored
// this cycle in go/cmd/evolve/cmd_loop_chain_minbatch_test.go and
// cmd_loop_chain_inboxvalidity_test.go. Every one of
// those exercises the system under test — calling chainStartDecision /
// inboxPendingCount and driving runLoopChain end-to-end over a real temp-dir
// inbox — and asserts on the returned decision, count, skip list, exit code and
// emitted chain summary. None is a source-grep of production code (the cycle-85
// degenerate-predicate ban). RED now: `inboxPendingCount` still has its 2-value
// signature and `chainStartDecision` still stops at n==0, so go/cmd/evolve does
// not compile / the assertions fail.
//
// The single doc predicate (004) asserts on a DOC artifact, which is inherently
// textual; it is a documentation-content criterion, not a behavioural one, and
// the behaviour it describes is independently pinned by 001–003.
package cycle1098

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// loopPkg is the chain loop's home package (package main — importable only via
// `go test`, hence the subprocess form).
const loopPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (e.g. the old 2-value inboxPendingCount signature) surfaces
// as a non-zero exit — the intended RED signal before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1098_001_ChainRunsAtLeastOneBatch — chain-min-one-batch AC1.1/AC1.4.
// Drives the pure decision function and the end-to-end chain: a drained inbox at
// n==0 STARTS a batch (the pre-chain single-batch contract survives opting into
// chaining), the chain runs exactly one batch and exits rc=0 with
// chain_inbox_empty, and the allowance is scoped to n==0 so a drained inbox
// after >=1 batch still stops cleanly (kills the blanket "never stop on empty"
// no-op).
func TestC1098_001_ChainRunsAtLeastOneBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestChainStartDecision_MinOneBatchOnDrainedInbox|TestChainStartDecision_DrainedInboxStopsAfterFirstBatch|TestRunLoopChain_DrainedInboxRunsExactlyOneBatch")
	if !ok {
		t.Errorf("chain mode still exits having run ZERO batches against a drained inbox "+
			"(or the min-one-batch allowance leaked past n==0):\n%s", out)
	}
}

// TestC1098_002_MinOneBatchPreservesBrakeAndCapPrecedence — chain-min-one-batch
// AC1.2/AC1.3, the NEGATIVE half. The allowance must not outrank the operator
// brake (`.evolve/loop-stop` at n==0 ⇒ zero batches, both as a decision and
// end-to-end) and must never widen the cap to cap+1 (a non-positive cap still
// runs nothing). Proves the fix is a scoped re-ordering, not "always run".
func TestC1098_002_MinOneBatchPreservesBrakeAndCapPrecedence(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestChainStartDecision_BrakeStillWinsAtZeroBatches|TestChainStartDecision_ZeroCapNeverRunsABatch|TestRunLoopChain_PreEngagedBrakeRunsZeroBatchesOnDrainedInbox")
	if !ok {
		t.Errorf("min-one-batch broke the pre-batch precedence — the operator brake is masked "+
			"or the max_batches ceiling is no longer exact:\n%s", out)
	}
}

// TestC1098_003_InboxPendingCountsRealItemsAndReportsSkips —
// chain-inbox-pending-validity AC2.1–AC2.4. Drives inboxPendingCount over real
// temp-dir fixtures (well-formed items, truncated JSON, 0-byte, top-level array,
// object without `id`, blank `id`, lifecycle subdir, non-json) and the chain
// end-to-end: only real items count, every skipped `*.json` is named (and
// surfaces on the chain's stderr), a missing inbox is still (0,nil), and a
// malformed-only inbox does NOT burn the chain to the cap on a false signal.
func TestC1098_003_InboxPendingCountsRealItemsAndReportsSkips(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestInboxPendingCount_SkipsMalformedAndNamesThem|TestInboxPendingCount_MissingInboxIsZeroWithNoSkips|TestInboxPendingCount_MalformedShapes|TestInboxPendingCount_ValidItemFixtureIsRepresentative|TestRunLoopChain_MalformedOnlyInboxDoesNotBurnToCap")
	if !ok {
		t.Errorf("inbox pending count still trusts every *.json (false pending signal burns the chain "+
			"to max_batches) or swallows skips silently:\n%s", out)
	}
}

// TestC1098_004_ChainRegressionSuiteStillGreen — anti-regression across BOTH
// tasks: the cycle-1075 chain contract (drain→next batch, quota defer, exact
// cap, brake, fleet-width preservation, rc mapping, CLI opt-in) must still hold
// after the two fixes. This is the predicate that fails if Builder "fixes" the
// defects by weakening the landed behaviour.
func TestC1098_004_ChainRegressionSuiteStillGreen(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_.*|TestChain.*Decision.*|TestInboxPendingCount.*|TestParseLoopArgs_UntilInboxEmpty")
	if !ok {
		t.Errorf("the cycle-1075 chain contract regressed while fixing cycle-1098's two defects:\n%s", out)
	}
}

// TestC1098_005_RuntimeReferenceDocumentsNewContract — the documentation
// criterion for both tasks (targetFiles names docs/operations/runtime-reference.md
// in each). Doc-content criteria are inherently textual; this predicate asserts
// on a DOC artifact, not on production source, and the behaviour it describes is
// independently pinned by 001–003 above.
//
// acs-predicate: doc-artifact-check
func TestC1098_005_RuntimeReferenceDocumentsNewContract(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("runtime-reference.md missing at %s", doc)
	}
	// The chaining row must state the min-one-batch guarantee...
	if !acsassert.LineContainsAll(doc, "Batch chaining", "at least one batch") {
		t.Errorf("the Batch chaining row does not document the min-one-batch guarantee — " +
			"operators read this row to know whether a drained-inbox chain does any work")
	}
	// ...and that non-item *.json files are skipped and reported, not counted.
	if !acsassert.LineContainsAll(doc, "Batch chaining", "skipped") {
		t.Errorf("the Batch chaining row does not document that malformed inbox files are " +
			"skipped and reported — the doc still implies every *.json is pending work")
	}
}
