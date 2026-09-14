package shiperr

import (
	"strings"
	"testing"
)

// ADR-0103 unit 07 — the step vocabulary's key, the five other whitelisted
// Debug keys and the whitelist the ONE ship.error producer (core's
// emitShipError) projects into Event.Fields. All live here, the leaf the ship
// phase and core already import, so the landing stamps the keys and core
// reads them without importing the landing — and every key has ONE spelling
// (a producer respelling a key would otherwise still write ship-error.json
// while the field silently vanished from the signal line).
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
