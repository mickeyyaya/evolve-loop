//go:build acs

package cycle786

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-tags", "integration", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

func TestC786_001_trivial_rebase_carries_audit_forward(t *testing.T) {
	runGoTest(t, shipPkg, "^TestTrivialRebase_CarriesAuditForward$",
		[]string{"TestTrivialRebase_CarriesAuditForward"})
}
