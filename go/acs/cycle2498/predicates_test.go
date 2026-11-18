//go:build acs

package cycle2498

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var launchEnvTests = []string{
	"TestAgentRunBridge_EvolveBinaryDirOnPath",
	"TestAgentRunBridge_EvolveBinaryDirOnPathLeavesNoEmptyEntry",
	"TestAgentRunBridge_EvolveBinaryDirOnPathIsAddedOnce",
	"TestAgentRunBridge_EvolveBinaryDirOnPathWhenTheSeatLaunches",
}

func TestC2498_001_AgentRunLaunchEnvironmentPutsTheEvolveBinaryDirFirstOnPath(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", moduleDir, "test", "-count=1", "-v", "-run", "^TestAgentRunBridge_EvolveBinaryDirOnPath", "./cmd/evolve")

	if err != nil || code != 0 {
		t.Fatalf("RED: the launch environment tests fail, exit=%d err=%v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))
	}
	for _, name := range launchEnvTests {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass\n%s", name, acsassert.OutputTail(stdout, 30))
		}
	}
}
