//go:build acs

package cycle2158

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runBridgeIntegrationTest(t *testing.T, name string) (string, error) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := acsassert.ModuleDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-tags", "integration", "-count=1", "-timeout", "150s", "-v", "-run", "^"+name+"$", "./internal/bridge")
	cmd.Dir = mod
	out, runErr := cmd.CombinedOutput()
	return string(out), runErr
}

func requirePass(t *testing.T, name string) {
	t.Helper()
	out, err := runBridgeIntegrationTest(t, name)
	if err != nil {
		t.Errorf("RED: %s failed: %v\n%s", name, err, acsassert.OutputTail(out, 25))
	}
	if !strings.Contains(out, "--- PASS: "+name+" ") {
		t.Errorf("RED: %s did not report a pass\n%s", name, acsassert.OutputTail(out, 25))
	}
}

func TestC2158_001_AgentReturnsWhileADaemonChildHoldsStdout(t *testing.T) {
	requirePass(t, "TestAgentWaitDelay_ReturnsWhileADaemonChildHoldsStdout")
}

func TestC2158_002_NoPrivateTmuxServerOutlivesTheRow(t *testing.T) {
	requirePass(t, "TestAgentWaitDelay_NoPrivateTmuxServerOutlivesTheRow")
}

func TestC2158_003_EveryNewTopLevelTestSatisfiesThePackageParallelGuard(t *testing.T) {
	requirePass(t, "TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial")
}
