//go:build acs

package cycle2350

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgeImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func requireBridgeTestPasses(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", bridgeImportPath,
	)
	if code != 0 {
		t.Fatalf("RED: %s exited %d\n%s", name, code, acsassert.OutputTail(stdout+stderr, 30))
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("RED: %s never ran to PASS (renamed or filtered out)\n%s", name, acsassert.OutputTail(stdout+stderr, 30))
	}
}

func TestC2350_001_CancelEndsTheResendWaitWithinOnePollAsNotVerified(t *testing.T) {
	requireBridgeTestPasses(t, "TestVerifySubmitted_CancelDuringBackoffEndsTheWaitWithinOnePoll")
}

func TestC2350_002_CancelDuringFourthBackoffStopsFurtherSendKeys(t *testing.T) {
	requireBridgeTestPasses(t, "TestVerifySubmitted_CancelDuringFourthBackoffStopsFurtherSendKeys")
	requireBridgeTestPasses(t, "TestVerifySubmitted_CancelAlreadyPendingWhenTheWaitStartsSendsNothingMore")
}

func TestC2350_003_NudgeSiteWaitLoopCountsTheResendWindow(t *testing.T) {
	requireBridgeTestPasses(t, "TestNudgeSite_ResendWindowCountsInElapsedWaitTime")
}

func TestC2350_004_EveryNewBridgeTestIsParallelOrListedSerial(t *testing.T) {
	requireBridgeTestPasses(t, "TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial")
}
