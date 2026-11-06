package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

func TestWaveStatus_SeatHealth_ADeadSessionTheLastOneOnItsSocketReadsZombie(t *testing.T) {
	serverGone := map[string]error{
		"no server running":     errors.New("no server running on /private/tmp/tmux-501/" + seatSocket),
		"socket file is absent": errors.New("error connecting to /private/tmp/tmux-501/" + seatSocket + " (No such file or directory)"),
	}
	for name, listErr := range serverGone {
		t.Run(name, func(t *testing.T) {
			rig := newSeatRig(t)
			rig.openRun(t)
			rig.pane(t, true, 20*time.Second)
			rig.tmuxErr = listErr

			wantOneSeat(t, rig.statusJSON(t), "zombie")
		})
	}
}

func TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerNeverReadsZombie(t *testing.T) {
	unreachable := map[string]error{
		"list timed out":     errors.New("signal: killed: context deadline exceeded"),
		"tmux is not at all": errors.New(`exec: "tmux": executable file not found in $PATH`),
		"exit status only":   errors.New("exit status 1"),
	}
	for name, listErr := range unreachable {
		t.Run(name, func(t *testing.T) {
			rig := newSeatRig(t)
			rig.openRun(t)
			rig.pane(t, false, 20*time.Second)
			rig.tmuxErr = listErr

			wantOneSeat(t, rig.statusJSON(t), "blocked")
		})
	}
}

type realTmuxSeat struct {
	rig     *seatRig
	socket  string
	session string
}

func newRealTmuxSeat(t *testing.T) *realTmuxSeat {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed; the real-tmux journey cannot run")
	}
	realLister := bridgeSessionsTmux
	rig := newSeatRig(t)
	bridgeSessionsTmux = realLister
	seat := &realTmuxSeat{
		rig:     rig,
		socket:  fmt.Sprintf("evolve-seat-journey-%d-%d", os.Getpid(), time.Now().UnixNano()),
		session: "evolve-bridge-journey-c2172-tdd",
	}
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", seat.socket, "kill-server").Run() })
	return seat
}

func (s *realTmuxSeat) tmux(t *testing.T, args ...string) {
	t.Helper()
	full := append([]string{"-L", s.socket}, args...)
	if out, err := exec.Command("tmux", full...).CombinedOutput(); err != nil {
		t.Fatalf("tmux %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (s *realTmuxSeat) writePane(t *testing.T, busy bool) {
	t.Helper()
	s.rig.openRun(t)
	snap := panewatch.Snapshot{
		Session: s.session, Socket: s.socket, CLI: "claude-tmux", Agent: "tdd", Cycle: seatCycle, RunID: "01M6MYQZPXAV3M7Z6XGM8BG7A8",
		Busy: busy, ProgressHash: "h1", ProgressAt: waveTestClock.Add(-20 * time.Second), UpdatedAt: waveTestClock, WriterPID: os.Getpid(),
	}
	if err := panewatch.Write(s.rig.runDir, snap); err != nil {
		t.Fatal(err)
	}
}

func (s *realTmuxSeat) waitServerGone(t *testing.T) {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		if exec.Command("tmux", "-L", s.socket, "list-sessions").Run() != nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("the tmux server on socket %s never exited after its last session was killed", s.socket)
}

func (s *realTmuxSeat) wantSeat(t *testing.T, state string) {
	t.Helper()
	got := s.rig.statusJSON(t)
	if len(got) != 1 || got[0].State != state || got[0].Session != s.session {
		t.Fatalf("seats = %+v, want exactly one seat of session %s reading %q", got, s.session, state)
	}
}

func TestWaveStatus_SeatHealth_ARealTmuxSocketWhoseLastSessionDiedReadsZombie(t *testing.T) {
	seat := newRealTmuxSeat(t)
	seat.tmux(t, "new-session", "-d", "-s", seat.session, "sleep 600")
	seat.writePane(t, true)
	seat.wantSeat(t, "working")

	seat.tmux(t, "kill-session", "-t", seat.session)
	seat.waitServerGone(t)

	seat.wantSeat(t, "zombie")
}

func TestWaveStatus_SeatHealth_ARealTmuxSessionAmongOthersReadsBlockedAtItsPrompt(t *testing.T) {
	seat := newRealTmuxSeat(t)
	seat.tmux(t, "new-session", "-d", "-s", seat.session, "sleep 600")
	seat.tmux(t, "new-session", "-d", "-s", "evolve-bridge-journey-bystander", "sleep 600")
	seat.writePane(t, false)

	seat.wantSeat(t, "blocked")

	seat.tmux(t, "kill-session", "-t", seat.session)
	seat.wantSeat(t, "zombie")
}

func TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var callers []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && id.Name == "bridgeSessionsTmux" {
						callers = append(callers, filepath.Base(path)+":"+fn.Name.Name)
						return false
					}
					return true
				})
			}
		}
	}
	if len(callers) != 1 {
		t.Errorf("bridgeSessionsTmux is used by %d functions %v, want exactly one shared lister", len(callers), callers)
	}
}
