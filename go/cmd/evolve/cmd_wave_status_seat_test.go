package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	seatCycle   = 2172
	seatSocket  = "evolve-bridge-p7905"
	seatSession = "evolve-bridge-r01M6MYQZ-c2172-tdd-pid99259-n5-1793852530"
)

type seatRowJSON struct {
	Cycle   int    `json:"cycle"`
	Phase   string `json:"phase"`
	State   string `json:"state"`
	Session string `json:"session"`
}

type seatRig struct {
	plane    wavePlane
	runDir   string
	tmux     []string
	tmuxErr  error
	sessions []string
}

func newSeatRig(t *testing.T) *seatRig {
	t.Helper()
	p := newWavePlane(t)
	rec := wave.Record{Number: 82, RunID: "r82", CycleFloor: seatCycle - 2, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}
	if err := p.store().Save(rec); err != nil {
		t.Fatal(err)
	}
	rig := &seatRig{plane: p, runDir: filepath.Join(p.evolveDir(), "runs", fmt.Sprintf("cycle-%d", seatCycle))}
	rig.sessions = []string{fmt.Sprintf("evolve-bridge-unrelated-session\t%d", waveTestClock.Add(-time.Minute).Unix())}
	if err := os.MkdirAll(rig.runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prevTmux, prevSockets := bridgeSessionsTmux, bridgeLiveSockets
	bridgeSessionsTmux = func(_ context.Context, socket string) (string, error) {
		rig.tmux = append(rig.tmux, socket)
		if rig.tmuxErr != nil {
			return "", rig.tmuxErr
		}
		return strings.Join(rig.sessions, "\n") + "\n", nil
	}
	bridgeLiveSockets = func() []string { return nil }
	t.Cleanup(func() { bridgeSessionsTmux, bridgeLiveSockets = prevTmux, prevSockets })
	return rig
}

func (r *seatRig) openRun(t *testing.T) {
	t.Helper()
	if err := runlease.Write(r.runDir, runlease.Lease{RunID: "01M6MYQZPXAV3M7Z6XGM8BG7A8", OwnerPID: os.Getpid()}, waveTestClock); err != nil {
		t.Fatal(err)
	}
}

func (r *seatRig) pane(t *testing.T, busy bool, progressAgo time.Duration) {
	t.Helper()
	snap := panewatch.Snapshot{
		Session: seatSession, Socket: seatSocket, CLI: "claude-tmux", Agent: "tdd", Cycle: seatCycle, RunID: "01M6MYQZPXAV3M7Z6XGM8BG7A8",
		Busy: busy, ProgressHash: "h1", ProgressAt: waveTestClock.Add(-progressAgo), UpdatedAt: waveTestClock, WriterPID: os.Getpid(),
	}
	if err := panewatch.Write(r.runDir, snap); err != nil {
		t.Fatal(err)
	}
}

func (r *seatRig) sessionAlive() {
	r.sessions = append(r.sessions, fmt.Sprintf("%s\t%d", seatSession, waveTestClock.Add(-5*time.Second).Unix()))
}

func (r *seatRig) deliver(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.runDir, "test-report.md"), []byte("# TDD Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *seatRig) statusJSON(t *testing.T) []seatRowJSON {
	t.Helper()
	h := newWaveHarness()
	if rc := h.run("status", "--json", "--project-root", r.plane.root); rc != 0 {
		t.Fatalf("wave status exit %d; stderr=%s", rc, h.stderr.String())
	}
	var out struct {
		Seats []seatRowJSON `json:"seats"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatalf("wave status --json is not JSON: %v\n%s", err, h.stdout.String())
	}
	return out.Seats
}

func (r *seatRig) statusText(t *testing.T) string {
	t.Helper()
	h := newWaveHarness()
	if rc := h.run("status", "--project-root", r.plane.root); rc != 0 {
		t.Fatalf("wave status exit %d; stderr=%s", rc, h.stderr.String())
	}
	return h.stdout.String()
}

func wantOneSeat(t *testing.T, got []seatRowJSON, state string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("seats = %+v, want exactly one seat reading %q", got, state)
	}
	if got[0].State != state || got[0].Phase != "tdd" || got[0].Cycle != seatCycle || got[0].Session != seatSession {
		t.Errorf("seat = %+v, want cycle %d phase tdd session %s state %q", got[0], seatCycle, seatSession, state)
	}
}

func seatLine(text, state string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "seat:") && strings.Contains(line, "tdd") && strings.Contains(line, state) {
			return true
		}
	}
	return false
}

func TestWaveStatus_SeatHealth_APaneAtItsPromptWithNoDeliverableReadsBlocked(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.sessionAlive()

	wantOneSeat(t, rig.statusJSON(t), "blocked")
	if text := rig.statusText(t); !seatLine(text, "blocked") {
		t.Errorf("text status has no seat line naming tdd and blocked:\n%s", text)
	}
}

func TestWaveStatus_SeatHealth_ADeadSessionWithAnOpenRunReadsZombie(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, true, 20*time.Second)

	wantOneSeat(t, rig.statusJSON(t), "zombie")
	if text := rig.statusText(t); !seatLine(text, "zombie") {
		t.Errorf("text status has no seat line naming tdd and zombie:\n%s", text)
	}
}

func TestWaveStatus_SeatHealth_ADeliverableMakesTheSeatIdleEvenAtItsPrompt(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.sessionAlive()
	rig.deliver(t)

	wantOneSeat(t, rig.statusJSON(t), "idle")
}

func TestWaveStatus_SeatHealth_ADeliverableOutranksADeadSession(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.deliver(t)

	wantOneSeat(t, rig.statusJSON(t), "idle")
}

func TestWaveStatus_SeatHealth_ABusyPaneWithFreshOutputReadsWorking(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, true, 5*time.Second)
	rig.sessionAlive()

	wantOneSeat(t, rig.statusJSON(t), "working")
}

func TestWaveStatus_SeatHealth_ABusyPaneSilentForHoursReadsStalled(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, true, 3*time.Hour)
	rig.sessionAlive()

	wantOneSeat(t, rig.statusJSON(t), "stalled")
}

func TestWaveStatus_SeatHealth_AnUnreachableSocketNeverReadsZombie(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.tmuxErr = errors.New("signal: killed: context deadline exceeded")

	wantOneSeat(t, rig.statusJSON(t), "blocked")
}

func TestWaveStatus_SeatHealth_AClosedRunShowsNoSeat(t *testing.T) {
	rig := newSeatRig(t)
	rig.pane(t, false, 20*time.Second)

	if got := rig.statusJSON(t); len(got) != 0 {
		t.Errorf("a run with no live lease shows seats %+v, want none", got)
	}
	if text := rig.statusText(t); strings.Contains(text, "seat:") {
		t.Errorf("a run with no live lease prints a seat line:\n%s", text)
	}
}

func TestWaveStatus_SeatHealth_TheViewReadsTmuxThroughTheListSeamAndChangesNothing(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.sessionAlive()
	before := treeDigest(t, rig.plane.root)

	wantOneSeat(t, rig.statusJSON(t), "blocked")
	_ = rig.statusText(t)

	if len(rig.tmux) == 0 {
		t.Fatal("the seat view never asked tmux for the live sessions, so it cannot tell zombie from blocked")
	}
	for _, socket := range rig.tmux {
		if socket != seatSocket {
			t.Errorf("the seat view asked tmux socket %q, want only the seat's own %q", socket, seatSocket)
		}
	}
	if after := treeDigest(t, rig.plane.root); after != before {
		t.Error("wave status changed files under the project root; the seat view must be read-only")
	}
}
