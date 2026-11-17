//go:build acs

package cycle2462

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func integrationRun(t *testing.T, count, pattern string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-tags", "integration", "-count="+count, "-run", pattern, "./cmd/evolve/")
	if code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	return stdout + stderr, code
}

func TestC2462_001_GCSuccessCaseLeavesNoGoTelemetryUnderTheTestHome(t *testing.T) {
	out, code := integrationRun(t, "1", "^TestCLIContract_GCSuccessLeavesNoGoTelemetryUnderTheTestHome$")
	if code != 0 {
		t.Errorf("gc/success wrote go telemetry under the test HOME (exit %d)\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("the telemetry test did not run\n%s", out)
	}
}

func TestC2462_002_GCContractCaseSurvivesFiftyRepeats(t *testing.T) {
	out, code := integrationRun(t, "50", "^TestCLIContract_EachCoreVerbPrintsOneEnvelope$/^gc$")
	if code != 0 {
		t.Errorf("-count=50 of the gc contract case failed (exit %d)\n%s", code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("the gc contract case did not run\n%s", out)
	}
}
