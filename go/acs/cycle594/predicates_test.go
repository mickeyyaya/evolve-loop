//go:build acs

package cycle594

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
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

func TestC594_001_MemoPinWithinEnvelope(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope drift not resolved in shipped config (config-only fix, no Go literals):\n%s", out)
	}
}

func TestC594_002_EnvelopeEnforcementIntact(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("envelope enforcement gutted — ValidatePin no longer rejects an out-of-envelope pin:\n%s", out)
	}
}

func TestC594_003_PostShipObserverNonFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestPostShipObserverSkip_MemoAfterShipIsNonFatal|TestPostShipObserverSkip_MandatoryFloorNeverSkipped|TestPostShipObserverSkip_ShipItselfNeverSkipped")
	if !ok {
		t.Errorf("post-ship observer failure not classified non-fatal (a shipped cycle can still be turned abnormal by a memo error):\n%s", out)
	}
}

func TestC594_004_PreShipFailureStaysFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestPostShipObserverSkip_MemoBeforeShipStaysFatal")
	if !ok {
		t.Errorf("pre-ship observer failure wrongly swallowed — the non-fatal downgrade must apply only once ship has landed:\n%s", out)
	}
}
