// reap_orphans_test.go — crash-recovery orphan GC contract.
//
// The registry reaper (reap_runsessions.go) only kills sessions a LIVE run
// recorded in its own per-run file; a SIGKILL'd loop never reaps, and the next
// loop can't reap the corpse (it's not in the new run's registry). This GC
// closes that gap by reaping sessions whose CREATOR PID is dead — and it must
// do so without reintroducing the 2026-06-11 killer-B class (a live concurrent
// run's sessions must survive). These tests pin both halves: reap the dead,
// never touch the living.
package swarm

import (
	"context"
	"errors"
	"testing"
)

// fakeServer is an injected SessionLister returning a fixed name set.
func fakeServer(names ...string) SessionLister {
	return func(_ context.Context) ([]string, error) { return names, nil }
}

// aliveSet is an injected PidLiveness: every pid in the set is "alive".
func aliveSet(pids ...int) PidLiveness {
	live := map[int]bool{}
	for _, p := range pids {
		live[p] = true
	}
	return func(pid int) bool { return live[pid] }
}

func recordingTmuxKiller() (TmuxKiller, *[]string) {
	var killed []string
	k := func(_ context.Context, s string) error {
		killed = append(killed, s)
		return nil
	}
	return k, &killed
}

func TestSessionPID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		session string
		wantPID int
		wantOK  bool
	}{
		{"build session", "evolve-bridge-r01KVZ0PV-c394-build-pid60161-nm-1782380224", 60161, true},
		{"recipe probe", "evolve-recipe-c0-usage-probe-pid165-n4-1782342570", 165, true},
		{"plain probe", "evolve-bridge-c0-probe-pid98167-n1-1782380671", 98167, true},
		{"pid at end", "evolve-bridge-codex-c0-probe-pid50395", 50395, true},
		{"no pid token (test session)", "evolve-bridge-it-iperm-71243", 0, false},
		{"no pid token at all", "evolve-bridge-something-else", 0, false},
		{"zero pid refused", "evolve-bridge-c1-x-pid0-n1-1", 0, false},
		{"rapid is not pid", "evolve-bridge-rapid7-c1-build", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pid, ok := SessionPID(tc.session)
			if ok != tc.wantOK || pid != tc.wantPID {
				t.Fatalf("SessionPID(%q) = (%d,%v), want (%d,%v)", tc.session, pid, ok, tc.wantPID, tc.wantOK)
			}
		})
	}
}

// TestReapOrphans_KillsDeadKeepsLive is THE acceptance: a crashed run's
// dead-pid session is reaped; a concurrent live run's session (live pid) is
// NEVER killed — the killer-B isolation guarantee, here enforced by liveness
// rather than by per-run file scoping.
func TestReapOrphans_KillsDeadKeepsLive(t *testing.T) {
	t.Parallel()
	const deadOrphan = "evolve-bridge-rDEAD0000-c394-build-pid60161-nm-1"
	const liveRun = "evolve-bridge-rLIVE1111-c5-build-pid4242-na-2"
	list := fakeServer(deadOrphan, liveRun)
	kill, killed := recordingTmuxKiller()

	rep := ReapOrphanSessions(context.Background(), list, aliveSet(4242), kill)

	if len(*killed) != 1 || (*killed)[0] != deadOrphan {
		t.Fatalf("killed=%v, want exactly the dead orphan %q", *killed, deadOrphan)
	}
	for _, s := range *killed {
		if s == liveRun {
			t.Fatal("reaped a LIVE concurrent run's session — killer-B class regression")
		}
	}
	if rep.SkippedLive != 1 {
		t.Fatalf("SkippedLive=%d, want 1 (the live run)", rep.SkippedLive)
	}
	if len(rep.Killed) != 1 {
		t.Fatalf("rep.Killed=%v, want 1", rep.Killed)
	}
}

func TestReapOrphans_SafetySkips(t *testing.T) {
	t.Parallel()
	foreign := "some-other-tmux-session"    // not in evolve namespace
	empty := ""                             // killer-B suicide class
	noPID := "evolve-bridge-it-iperm-71243" // can't reason about liveness
	deadBridge := "evolve-bridge-c1-build-pid900-n1-1"
	deadRecipe := "evolve-recipe-c0-usage-probe-pid901-n1-1"
	list := fakeServer(foreign, empty, noPID, deadBridge, deadRecipe)
	kill, killed := recordingTmuxKiller()

	rep := ReapOrphanSessions(context.Background(), list, aliveSet(), kill)

	if rep.SkippedForeign != 2 { // foreign + empty
		t.Fatalf("SkippedForeign=%d, want 2 (foreign + empty)", rep.SkippedForeign)
	}
	if rep.SkippedUnparseable != 1 {
		t.Fatalf("SkippedUnparseable=%d, want 1 (no-pid test session)", rep.SkippedUnparseable)
	}
	if len(*killed) != 2 {
		t.Fatalf("killed=%v, want exactly the 2 dead evolve sessions", *killed)
	}
	for _, s := range *killed {
		if s == foreign || s == empty || s == noPID {
			t.Fatalf("killed an unsafe session %q — safety gate broken", s)
		}
	}
}

// TestReapOrphans_KillErrorContinues: a kill failure on one session is recorded
// but the sweep proceeds to its siblings (an orphan must never block reaping
// the rest).
func TestReapOrphans_KillErrorContinues(t *testing.T) {
	t.Parallel()
	const bad = "evolve-bridge-c1-build-pid111-n1-1"
	const good = "evolve-bridge-c1-audit-pid222-n1-2"
	list := fakeServer(bad, good)
	var killed []string
	kill := func(_ context.Context, s string) error {
		if s == bad {
			return errors.New("tmux boom")
		}
		killed = append(killed, s)
		return nil
	}

	rep := ReapOrphanSessions(context.Background(), list, aliveSet(), kill)

	if len(rep.Errors) != 1 {
		t.Fatalf("Errors=%v, want 1", rep.Errors)
	}
	if len(killed) != 1 || killed[0] != good {
		t.Fatalf("killed=%v, want the good session despite the bad one erroring", killed)
	}
	if len(rep.Killed) != 1 || rep.Killed[0] != good {
		t.Fatalf("rep.Killed=%v, want only the successfully-killed session", rep.Killed)
	}
}

// TestReapOrphans_ListErrorIsSafe: an unreadable server surfaces an error and
// kills nothing (degrade to leak-on-failure, never a blind sweep).
func TestReapOrphans_ListErrorIsSafe(t *testing.T) {
	t.Parallel()
	list := func(_ context.Context) ([]string, error) { return nil, errors.New("no server") }
	kill, killed := recordingTmuxKiller()

	rep := ReapOrphanSessions(context.Background(), list, aliveSet(), kill)

	if len(*killed) != 0 {
		t.Fatalf("killed=%v on list error, want none", *killed)
	}
	if len(rep.Errors) != 1 {
		t.Fatalf("Errors=%v, want 1 list error", rep.Errors)
	}
}
