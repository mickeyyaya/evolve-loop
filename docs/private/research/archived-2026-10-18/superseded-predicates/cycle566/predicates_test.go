//go:build acs

package cycle566

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const profilesPkg = "github.com/mickeyyaya/evolve-loop/go/internal/profiles"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC566_002_EffortDefaultMatrix(t *testing.T) {
	ok, out := runGoTest(t, profilesPkg, "TestEffortDefaults_Matrix")
	if !ok {
		t.Errorf("per-phase effort default matrix not pinned in config (scout/triage=low, tdd/audit/adversarial=medium, builder=medium):\n%s", out)
	}
}
