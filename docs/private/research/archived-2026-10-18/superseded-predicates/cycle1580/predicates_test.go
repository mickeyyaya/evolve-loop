//go:build acs

package cycle1580

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

func runPkgTests(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively(t *testing.T) {
	ok, out := runPkgTests(t, deliverablePkg, "TestVerify_Scout_MissingChallengeToken_Violation|"+
		"TestVerify_Scout_WrongToken_Violation|"+
		"TestVerify_Scout_TokenEchoed_OK|"+
		"TestVerify_Scout_NoTokenFile_FailOpen|"+
		"TestContract_RequireChallengeToken_CoversTokenConsumingPhases")
	if !ok {
		t.Errorf("AC-R1 unmet: the scout challenge-token binding is not enforced at the pre-audit contract gate — it still fails open on persona prose\n%s", out)
	}
}
