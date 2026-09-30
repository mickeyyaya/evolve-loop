//go:build acs

package cycle1279

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const goTestTimeout = 4 * time.Minute

func runNamedTests(t *testing.T, pkg, pattern string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), goTestTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func requirePassing(t *testing.T, pkg, pattern string, names ...string) {
	t.Helper()
	out, ok := runNamedTests(t, pkg, pattern)
	if !ok {
		t.Errorf("%s: `go test -run %s` failed:\n%s", pkg, pattern, tail(out))
		return
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("%s: %s did not run to a PASS (a -run pattern that matches nothing also exits 0):\n%s", pkg, n, tail(out))
		}
	}
}

func tail(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

func TestC1279_001_RejectingAuditEmitsDefectLedger(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(RejectingAuditEmitsDefectLedger|PassingAuditWritesNoLedger)$",
		"TestClassify_RejectingAuditEmitsDefectLedger",
		"TestClassify_PassingAuditWritesNoLedger",
	)
}

func TestC1279_002_ContinuationCannotLaunderADefect(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_Continuation(CannotPassWithUnaccountedDefect|WithNoDispositionArtifactCannotPass)$",
		"TestClassify_ContinuationCannotPassWithUnaccountedDefect",
		"TestClassify_ContinuationWithNoDispositionArtifactCannotPass",
	)
}

func TestC1279_003_DispositionsAreVisibleAndEntriesSurvive(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_ContinuationLedgerRetainsEveryEntry$",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
	)
}

func TestC1279_004_NonContinuationPassPathUnperturbed(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_NonContinuationPassPathUnchanged$",
		"TestClassify_NonContinuationPassPathUnchanged",
	)
}

func TestC1279_005_RetroRemediationReachesTheInbox(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^TestWriteArtifacts_(InboxItemsLandBesideRetrospective|InboxFailureLeavesNoRetrospective)$",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective",
	)
}

func TestC1279_006_ExistingFaillearnCallersUnchanged(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^Test",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
		"TestWriteArtifacts_EmptyInboxItemsMintsNoFiles",
	)
}
