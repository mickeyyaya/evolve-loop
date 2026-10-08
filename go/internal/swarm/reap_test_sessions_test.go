package swarm

import (
	"context"
	"reflect"
	"testing"
)

func TestReapOrphans_ALeakedTestSessionWhoseTestProcessIsDeadIsReaped(t *testing.T) {
	t.Parallel()
	leaked := "evolve-bridge-it-cc2-3948"
	running := "evolve-bridge-it-iperm-71243"
	noPid := "evolve-bridge-it-iperm"
	init := "evolve-bridge-it-x-1"
	list := fakeServer(leaked, running, noPid, init)
	kill, killed := recordingTmuxKiller()

	rep := ReapOrphanSessions(context.Background(), list, aliveSet(71243), kill)

	if !reflect.DeepEqual(*killed, []string{leaked}) {
		t.Fatalf("killed=%v, want only %q: its test process 3948 is dead", *killed, leaked)
	}
	if rep.SkippedLive != 1 || rep.SkippedUnparseable != 2 {
		t.Errorf("SkippedLive=%d SkippedUnparseable=%d, want 1 (the running test) and 2 (no pid, pid 1)", rep.SkippedLive, rep.SkippedUnparseable)
	}
}
