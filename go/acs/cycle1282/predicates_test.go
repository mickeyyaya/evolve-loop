//go:build acs

package cycle1282

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

func TestC1282_001_LedgerNeverShrinksAndIDsAreStable(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(ContinuationRetryDoesNotEraseOwnEntries|LedgerIDsAreContentDerived)$",
		"TestClassify_ContinuationRetryDoesNotEraseOwnEntries",
		"TestClassify_LedgerIDsAreContentDerived",
	)
}

func TestC1282_002_MissingAncestorLedgerIsDiagnosed(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(ContinuationWithNoAncestorLedgerIsDiagnosed|NonContinuationEmitsNoLedgerDiagnostic)$",
		"TestClassify_ContinuationWithNoAncestorLedgerIsDiagnosed",
		"TestClassify_NonContinuationEmitsNoLedgerDiagnostic",
	)
}

func TestC1282_003_ClosureEvidenceMustResolve(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(UnresolvableEvidenceDoesNotCloseADefect|ResolvableEvidenceClosesADefect)$",
		"TestClassify_UnresolvableEvidenceDoesNotCloseADefect",
		"TestClassify_ResolvableEvidenceClosesADefect",
	)
}

func TestC1282_004_EveryDispositionArmIsExercised(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_(DispositionArms|CarriesForwardAlreadyDispositionedAncestorEntry|ContinuationLedgerRetainsEveryEntry)$",
		"TestClassify_DispositionArms",
		"TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
	)
}

func TestC1282_005_EmitCoversWarnNotFailAlone(t *testing.T) {
	requirePassing(t,
		"./internal/phases/audit",
		"^TestClassify_Warn(WithStructuredDefectsEmitsLedger|WithoutStructuredDefectsMintsNothing)$",
		"TestClassify_WarnWithStructuredDefectsEmitsLedger",
		"TestClassify_WarnWithoutStructuredDefectsMintsNothing",
	)
}

func TestC1282_006_DegenerateEchoIsNotFiledToInbox(t *testing.T) {
	const pattern = "^TestWriteDeterministicLearning_(ClassedButDefectlessBlockFilesNothing|EchoDefectListFilesNothing|StructuredDefectsAreFiled)$"
	out, ok := runNamedTests(t, "./internal/core", pattern)
	if !ok {
		t.Errorf("./internal/core: `go test -run %s` failed:\n%s", pattern, tail(out))
		return
	}
	for _, n := range []string{
		"TestWriteDeterministicLearning_ClassedButDefectlessBlockFilesNothing",
		"TestWriteDeterministicLearning_EchoDefectListFilesNothing",
		"TestWriteDeterministicLearning_StructuredDefectsAreFiled",
	} {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("./internal/core: %s did not run to a PASS (a -run pattern that matches nothing also exits 0):\n%s", n, tail(out))
		}
	}
}

func TestC1282_007_InboxIDCannotEscapeTheInboxDirectory(t *testing.T) {
	requirePassing(t,
		"./internal/faillearn",
		"^Test",
		"TestWriteArtifacts_InboxRejectsPathEscapingID",
		"TestWriteArtifacts_InboxAcceptsOrdinaryID",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
	)
}
