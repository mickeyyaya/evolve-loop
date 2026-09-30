//go:build acs

package cycle1285

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runContract(t *testing.T, pkg, pattern string, want ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-run", pattern, "-v", pkg)
	switch {
	case code > 0:
		t.Errorf("go test %s -run %s exited %d — the contract is RED\nstdout:\n%s\nstderr:\n%s",
			pkg, pattern, code, tail(stdout), tail(stderr))
		return
	case err != nil:
		t.Fatalf("go test %s -run %s: could not run: %v\nstderr:\n%s", pkg, pattern, err, tail(stderr))
	}
	for _, name := range want {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("no `--- PASS: %s` receipt in %s — the contract test is missing, renamed, or skipped; a `-run` pattern that matches nothing exits 0 and proves nothing\nstdout:\n%s",
				name, pkg, tail(stdout))
		}
	}
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}

func TestC1285_001_ClosureClaimCitationGate(t *testing.T) {
	runContract(t, "./internal/phases/audit", "TestC1285_4",
		"TestC1285_401_ClassifyBlocksUncitedClosureClaim",
		"TestC1285_402_ClassifyAllowsCitedClosureClaim",
		"TestC1285_403_OrdinaryReportUnaffected",
		"TestC1285_404_ClosureOffendersAreLineScoped",
	)
}

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

func TestC1285_003_RetroRemediationIsInboxTransactional(t *testing.T) {
	runContract(t, "./internal/faillearn", "TestWriteArtifacts_(InboxItemsLandBesideRetrospective|InboxFailureLeavesNoRetrospective|WithoutInboxOptionIsUnchanged|EmptyInboxItemsMintsNoFiles)",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
		"TestWriteArtifacts_EmptyInboxItemsMintsNoFiles",
	)
}

// acs-predicate: config-check — a documentation criterion has no runtime
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
