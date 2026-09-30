//go:build acs

package cycle1098

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const loopPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1098_001_ChainRunsAtLeastOneBatch(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestChainStartDecision_MinOneBatchOnDrainedInbox|TestChainStartDecision_DrainedInboxStopsAfterFirstBatch|TestRunLoopChain_DrainedInboxRunsExactlyOneBatch")
	if !ok {
		t.Errorf("chain mode still exits having run ZERO batches against a drained inbox "+
			"(or the min-one-batch allowance leaked past n==0):\n%s", out)
	}
}

func TestC1098_002_MinOneBatchPreservesBrakeAndCapPrecedence(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestChainStartDecision_BrakeStillWinsAtZeroBatches|TestChainStartDecision_ZeroCapNeverRunsABatch|TestRunLoopChain_PreEngagedBrakeRunsZeroBatchesOnDrainedInbox")
	if !ok {
		t.Errorf("min-one-batch broke the pre-batch precedence — the operator brake is masked "+
			"or the max_batches ceiling is no longer exact:\n%s", out)
	}
}

func TestC1098_003_InboxPendingCountsRealItemsAndReportsSkips(t *testing.T) {
	ok, out := runGoTest(t, loopPkg,
		"TestInboxPendingCount_SkipsMalformedAndNamesThem|TestInboxPendingCount_MissingInboxIsZeroWithNoSkips|TestInboxPendingCount_MalformedShapes|TestInboxPendingCount_ValidItemFixtureIsRepresentative|TestRunLoopChain_MalformedOnlyInboxDoesNotBurnToCap")
	if !ok {
		t.Errorf("inbox pending count still trusts every *.json (false pending signal burns the chain "+
			"to max_batches) or swallows skips silently:\n%s", out)
	}
}

func TestC1098_004_ChainRegressionSuiteStillGreen(t *testing.T) {
	ok, out := runGoTest(t, loopPkg, "TestRunLoopChain_.*|TestChain.*Decision.*|TestInboxPendingCount.*|TestParseLoopArgs_UntilInboxEmpty")
	if !ok {
		t.Errorf("the cycle-1075 chain contract regressed while fixing cycle-1098's two defects:\n%s", out)
	}
}

// acs-predicate: doc-artifact-check
func TestC1098_005_RuntimeReferenceDocumentsNewContract(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("runtime-reference.md missing at %s", doc)
	}
	if !acsassert.LineContainsAll(doc, "Batch chaining", "at least one batch") {
		t.Errorf("the Batch chaining row does not document the min-one-batch guarantee — " +
			"operators read this row to know whether a drained-inbox chain does any work")
	}
	if !acsassert.LineContainsAll(doc, "Batch chaining", "skipped") {
		t.Errorf("the Batch chaining row does not document that malformed inbox files are " +
			"skipped and reported — the doc still implies every *.json is pending work")
	}
}
