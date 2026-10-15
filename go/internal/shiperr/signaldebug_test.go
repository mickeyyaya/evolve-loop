package shiperr

import (
	"strings"
	"testing"
)

func TestShipErr_StepKeyAndSignalDebugKeys(t *testing.T) {
	consts := []string{StepKey, GitRCKey, WorktreeKey, BranchKey, CycleBranchKey, RepairOutcomeKey}
	wire := []string{"step", "git_rc", "worktree", "branch", "cycle_branch", "repair_outcome"}
	if strings.Join(consts, ",") != strings.Join(wire, ",") {
		t.Errorf("the Debug-key constants spell %v, want the wire strings %v", consts, wire)
	}
	if strings.Join(SignalDebugKeys, ",") != strings.Join(consts, ",") {
		t.Errorf("SignalDebugKeys = %v, want exactly the constants %v in order (a key is added as a constant, in this list and in its doc together)", SignalDebugKeys, consts)
	}
	if SignalDebugKeys[0] != StepKey {
		t.Error("the step key heads the whitelist")
	}
	se := NewShipError(CodeGitPushRejected, ShipClassTransient, StageAtomicShip, "m", StepKey, "push", BranchKey, "main")
	if se.Debug[StepKey] != "push" || se.Debug[BranchKey] != "main" || !strings.Contains(se.DebugString(), "step=push") {
		t.Errorf("the keys ride the Debug map and DebugString: %v %q", se.Debug, se.DebugString())
	}
}
