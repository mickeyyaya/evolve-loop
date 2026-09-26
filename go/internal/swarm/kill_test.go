package swarm

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

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
	_ = reg.MarkReaped("w1")

	fk := &fakeKiller{}
	rep := Reap(context.Background(), reg, fk)

	if len(rep.Killed) != 2 {
		t.Fatalf("want 2 killed (w0,w2), got %v", rep.Killed)
	}
	if len(reg.Live()) != 0 {
		t.Errorf("no sessions should remain live, got %v", reg.Live())
	}
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
	if err := k.Kill(context.Background(), SessionHandle{WorkerID: "w0"}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("killer must not act on a zero pgid / empty tmux session")
	}
}

func TestExecSessionKiller_KillGroupError_ContinuesToKillTmux(t *testing.T) {
	var tmuxCalled bool
	k := ExecSessionKiller{
		KillGroup: func(int) error { return errors.New("no such process") },
		KillTmux:  func(_ context.Context, _ string) error { tmuxCalled = true; return nil },
	}
	err := k.Kill(context.Background(), SessionHandle{PGID: 100, TmuxSession: "sess-w0"})
	if err == nil {
		t.Error("KillGroup error must propagate as the return value")
	}
	if !tmuxCalled {
		t.Error("KillTmux must still be called even when KillGroup errors (both steps are best-effort)")
	}
}

func TestExecSessionKiller_KillTmuxError_ReturnsErr(t *testing.T) {
	k := ExecSessionKiller{
		KillTmux: func(context.Context, string) error { return errors.New("session not found") },
	}
	err := k.Kill(context.Background(), SessionHandle{TmuxSession: "dead-sess"})
	if err == nil {
		t.Error("KillTmux error must propagate as the return value")
	}
}

func TestExecSessionKiller_RejectsPGID1(t *testing.T) {
	called := false
	k := ExecSessionKiller{
		KillGroup: func(int) error { called = true; return nil },
	}
	_ = k.Kill(context.Background(), SessionHandle{PGID: 1})
	if called {
		t.Error("KillGroup must NOT be called for PGID 1 (init protection)")
	}
}
