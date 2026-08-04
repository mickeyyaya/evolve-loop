package bridge

// driver_tmux_repl_fleet_accept_test.go — cycle-1270 Task 2
// (`retro-fleet-worktree-dispatch`), the missing POSITIVE half.
//
// Both halves of the fleet-worktree contract are tested today and neither
// proves the contract holds: retro proves it MINTS a scratch cwd
// (phases/retro/retro_worktree_fallback_test.go), and this package proves the
// guard REFUSES an empty one (TestFleetModeRefusesEmptyWorktree). Nothing
// proves a minted directory actually CLEARS the guard.
//
// A future tightening of that guard (e.g. requiring a .git entry) would break
// every fleet-lane retro with both existing suites still green — the exact
// silent-regression shape the item exists to close. The pair IS the contract:
// accepts a real owned cwd, refuses an empty one.

import (
	"context"
	"testing"
)

func TestFleetModeAcceptsScratchCwdWorktree(t *testing.T) {
	scratch := t.TempDir() // the shape retro mints: a real, owned, writable dir
	cfg := fixtureConfig(t)
	cfg.Worktree = scratch
	tm := &workdirRecordingTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}}
	deps := fixtureDeps(tm)
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_FLEET": "1"})

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "fleet-accept", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})

	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil — fleet mode must ACCEPT an explicitly designated, existing worktree. "+
			"Refusing one strands every lane whose retro was dispatched over a minted scratch cwd, and the fix belongs on the phase side (retro.go), never in a widened guard", code, err)
	}
	if tm.bornIn != scratch {
		t.Errorf("session born in %q, want %q — the designated worktree must be the launch's working dir, not merely accepted and then ignored", tm.bornIn, scratch)
	}
}
