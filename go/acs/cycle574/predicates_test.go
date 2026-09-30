//go:build acs

package cycle574

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC574_001_MemoTierEnvelopeAligned(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope|TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope alignment regressed (config-only fix, no Go literals):\n%s", out)
	}
}

func TestC574_002_MemoPostShipFailureNonFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestPostShipObserverSkip_MemoAfterShipIsNonFatal")
	if !ok {
		t.Errorf("post-ship memo failure still flips a shipped cycle abnormal — the non-fatal observer classifier is missing:\n%s", out)
	}
}

func TestC574_003_PostShipSkipScopedAndFloorSafe(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestPostShipObserverSkip_MemoBeforeShipStaysFatal|TestPostShipObserverSkip_MandatoryFloorNeverSkipped|TestPostShipObserverSkip_ShipItselfNeverSkipped")
	if !ok {
		t.Errorf("post-ship observer skip is mis-scoped (swallows pre-ship failures, or skips a floor/anchor phase):\n%s", out)
	}
}
