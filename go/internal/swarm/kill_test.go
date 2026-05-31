package swarm

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// fakeKiller records what it was asked to kill and can be set to fail.
type fakeKiller struct {
	killed []string
	failOn map[string]bool
}

func (f *fakeKiller) Kill(_ context.Context, h SessionHandle) error {
	f.killed = append(f.killed, h.WorkerID)
	if f.failOn[h.WorkerID] {
		return errors.New("boom")
	}
	return nil
}

func TestReap_KillsAllLiveAndMarksReaped(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = reg.Register(handle("w0"))
	_ = reg.Register(handle("w1"))
	_ = reg.Register(handle("w2"))
	_ = reg.MarkReaped("w1") // already reaped → should be skipped

	fk := &fakeKiller{}
	rep := Reap(context.Background(), reg, fk)

	if len(rep.Killed) != 2 {
		t.Fatalf("want 2 killed (w0,w2), got %v", rep.Killed)
	}
	if len(reg.Live()) != 0 {
		t.Errorf("no sessions should remain live, got %v", reg.Live())
	}
	// w1 was already reaped → killer never touched it.
	for _, id := range fk.killed {
		if id == "w1" {
			t.Errorf("already-reaped w1 must not be killed again")
		}
	}
}

func TestReap_ContinuesPastKillError(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = reg.Register(handle("w0"))
	_ = reg.Register(handle("w1"))

	fk := &fakeKiller{failOn: map[string]bool{"w0": true}}
	rep := Reap(context.Background(), reg, fk)

	if len(rep.Errors) != 1 {
		t.Errorf("want 1 error from w0, got %v", rep.Errors)
	}
	// Both still get marked reaped (a corpse must not block future sweeps).
	if len(reg.Live()) != 0 {
		t.Errorf("all sessions reaped despite error, got live %v", reg.Live())
	}
}

func TestExecSessionKiller_BothStepsBestEffort(t *testing.T) {
	var killedPGID int
	var killedTmux string
	k := ExecSessionKiller{
		KillGroup: func(pgid int) error { killedPGID = pgid; return nil },
		KillTmux:  func(_ context.Context, s string) error { killedTmux = s; return nil },
	}
	h := SessionHandle{WorkerID: "w0", PGID: 4242, TmuxSession: "sess-w0"}
	if err := k.Kill(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	if killedPGID != 4242 {
		t.Errorf("pgid not killed, got %d", killedPGID)
	}
	if killedTmux != "sess-w0" {
		t.Errorf("tmux not killed, got %q", killedTmux)
	}
}

func TestExecSessionKiller_SkipsZeroPGIDAndEmptyTmux(t *testing.T) {
	called := false
	k := ExecSessionKiller{
		KillGroup: func(int) error { called = true; return nil },
		KillTmux:  func(context.Context, string) error { called = true; return nil },
	}
	// No pgid, no tmux session → nothing to do, no error.
	if err := k.Kill(context.Background(), SessionHandle{WorkerID: "w0"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("killer must not act on a zero pgid / empty tmux session")
	}
}
