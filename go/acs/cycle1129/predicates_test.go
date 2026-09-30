//go:build acs

package cycle1129

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1129_001_DriftAlarmIgnoresAgentDiffContent(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_AgentDiffContent_NoFalseAlarm")
	if !ok {
		t.Errorf("the exit-81 drift alarm still fires on AGENT-AUTHORED diff content — driver_tmux_repl.go is passing the raw lastGoodPane instead of strippedForExhaustionScan:\n%s", out)
	}
}

func TestC1129_002_DriftAlarmIgnoresPromptEcho(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_PromptEchoContent_NoFalseAlarm")
	if !ok {
		t.Errorf("the exit-81 drift alarm still fires on an ECHOED PROMPT line — the teardown pane is not being run through strippedForExhaustionScan:\n%s", out)
	}
}

func TestC1129_003_DriftAlarmStillFiresOnRealDriftedWall(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_RealDriftedWallStillFires")
	if !ok {
		t.Errorf("the drift alarm no longer fires on a genuine drifted CLI wall — the 8-cycle-silent-burn diagnostic has been silenced rather than de-noised:\n%s", out)
	}
}

func TestC1129_004_ExistingDriftAlarmContractIntact(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestWarnExhaustionRegexDrift|TestClaudeTmuxDriftProbe_MatchesRealWall|TestDriftProbeArmedPerCLI")
	if !ok {
		t.Errorf("the pre-existing drift-alarm contract regressed — the call-site pane fix must not change the alarm's firing condition:\n%s", out)
	}
}

func TestC1129_005_ExhaustionDetectionUnregressed(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_ExhaustionWall_FailsOverFast|TestTmuxREPL_ExhaustionWall_FastFailNotAtCheckpoint|TestTmuxREPL_HealthyIdle_NotExhausted")
	if !ok {
		t.Errorf("the primary exhaustion fast-fail path regressed — stripping the drift alarm's pane must not touch the detector that already stripped:\n%s", out)
	}
}
