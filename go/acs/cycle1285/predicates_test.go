//go:build acs

// Package cycle1285 holds the cycle-1285 ACS predicates for the inbox item
// `continuation-defect-ledger` (batch-integrity-review-2026-08-04.md F1).
//
// Every behavioral predicate below runs the frozen in-package contract tests as
// a SUBPROCESS and requires the named `--- PASS: <test>` receipt. Two reasons
// for that shape rather than a source assertion:
//
//   - the mechanism under test (emitDefectLedger / reconcileContinuationDefects
//     / closureClaimOffenders) is unexported and reachable only through
//     hooks.Classify, so an out-of-package predicate cannot call it directly;
//   - requiring the NAMED per-test receipt closes `go test -run` returning 0
//     on "no tests to run", which is how a deleted contract test would
//     otherwise green this suite in silence.
//
// Each invocation names ONE package and narrows with -run (never a `./...`
// sweep, never ./internal/core or ./cmd/evolve) per the flaky-predicate rules.
package cycle1285

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// runContract executes `go -C <worktree>/go test -run <pattern> -v <pkg>` and
// fails unless every test in want reported `--- PASS`. Returns nothing: the
// receipt check IS the assertion.
func runContract(t *testing.T, pkg, pattern string, want ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-run", pattern, "-v", pkg)
	switch {
	case code > 0:
		// A real verdict from the toolchain: compile failure or a failing test.
		t.Errorf("go test %s -run %s exited %d — the contract is RED\nstdout:\n%s\nstderr:\n%s",
			pkg, pattern, code, tail(stdout), tail(stderr))
		return
	case err != nil:
		// Could not run at all (no toolchain, signal). Distinguished from the
		// case above so an environment gap never reads as a red contract.
		t.Fatalf("go test %s -run %s: could not run: %v\nstderr:\n%s", pkg, pattern, err, tail(stderr))
	}
	for _, name := range want {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("no `--- PASS: %s` receipt in %s — the contract test is missing, renamed, or skipped; a `-run` pattern that matches nothing exits 0 and proves nothing\nstdout:\n%s",
				name, pkg, tail(stdout))
		}
	}
}

// tail bounds pasted output so a failing predicate stays readable.
func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

// TestC1285_001_ClosureClaimCitationGate — inbox clause (3). A bookkeeping
// closure claim ("verified closed") that cites no per-defect disposition
// artifact must not be able to leave audit on PASS; a cited one must.
func TestC1285_001_ClosureClaimCitationGate(t *testing.T) {
	runContract(t, "./internal/phases/audit", "TestC1285_4",
		"TestC1285_401_ClassifyBlocksUncitedClosureClaim",
		"TestC1285_402_ClassifyAllowsCitedClosureClaim",
		"TestC1285_403_OrdinaryReportUnaffected",
		"TestC1285_404_ClosureOffendersAreLineScoped",
	)
}

// TestC1285_002_DefectLedgerEmitAndReconcile — inbox clause (1). A rejecting
// audit leaves addressable OPEN entries, and a continuation may not PASS while
// an inherited entry is unaccounted for or closed on evidence that resolves to
// nothing.
func TestC1285_002_DefectLedgerEmitAndReconcile(t *testing.T) {
	runContract(t, "./internal/phases/audit", "TestClassify_(RejectingAuditEmitsDefectLedger|PassingAuditWritesNoLedger|ContinuationCannotPassWithUnaccountedDefect|ContinuationWithNoDispositionArtifactCannotPass|UnresolvableEvidenceDoesNotCloseADefect|ContinuationLedgerRetainsEveryEntry)",
		"TestClassify_RejectingAuditEmitsDefectLedger",
		"TestClassify_PassingAuditWritesNoLedger",
		"TestClassify_ContinuationCannotPassWithUnaccountedDefect",
		"TestClassify_ContinuationWithNoDispositionArtifactCannotPass",
		"TestClassify_UnresolvableEvidenceDoesNotCloseADefect",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
	)
}

// TestC1285_003_RetroRemediationIsInboxTransactional — inbox clause (2). Retro
// remediation reaches `.evolve/inbox` in the SAME call as the retrospective,
// and a failed inbox write leaves no retrospective claiming filed work.
func TestC1285_003_RetroRemediationIsInboxTransactional(t *testing.T) {
	runContract(t, "./internal/faillearn", "TestWriteArtifacts_(InboxItemsLandBesideRetrospective|InboxFailureLeavesNoRetrospective|WithoutInboxOptionIsUnchanged|EmptyInboxItemsMintsNoFiles)",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
		"TestWriteArtifacts_EmptyInboxItemsMintsNoFiles",
	)
}

// TestC1285_004_LandingDocumentedIssueGapSolution — operating-policy §3.8 and
// the inbox item's DOCS REQUIRED clause: the landing carries an issue/gap/
// solution entry a reader can audit against the diff.
//
// acs-predicate: config-check — a documentation criterion has no runtime
// surface to exercise; the artifact's presence and shape IS the requirement.
func TestC1285_004_LandingDocumentedIssueGapSolution(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations",
		"batch-integrity-review-2026-08-04.md")
	for _, needle := range []string{
		"closureClaimOffenders",
		"defect-dispositions.json",
		"Issue",
		"Gap",
		"Solution",
	} {
		if !acsassert.FileContains(t, doc, needle) {
			t.Errorf("%s must document the cycle-1285 landing in issue/gap/solution format naming the gate and the artifact it requires (missing %q)", doc, needle)
		}
	}
}
