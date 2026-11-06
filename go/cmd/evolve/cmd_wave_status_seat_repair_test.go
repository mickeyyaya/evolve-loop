package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

type seatStatusDoc struct {
	Seats    []seatRowJSON `json:"seats"`
	Warnings []string      `json:"warnings"`
}

func (r *seatRig) statusDoc(t *testing.T) seatStatusDoc {
	t.Helper()
	h := newWaveHarness()
	if rc := h.run("status", "--json", "--project-root", r.plane.root); rc != 0 {
		t.Fatalf("wave status exit %d; stderr=%s", rc, h.stderr.String())
	}
	var doc seatStatusDoc
	if err := json.Unmarshal(h.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("wave status --json is not JSON: %v\n%s", err, h.stdout.String())
	}
	return doc
}

func (r *seatRig) dispatchBeganAt(t *testing.T, began time.Time, busy bool) {
	t.Helper()
	tracker := panewatch.NewTracker(panewatch.Snapshot{
		Session: seatSession, Socket: seatSocket, CLI: "claude-tmux", Agent: "tdd", Cycle: seatCycle,
		RunID: "01M6MYQZPXAV3M7Z6XGM8BG7A8", WriterPID: os.Getpid(),
	})
	tracker.Observe(panewatch.Frame{Hash: "first-frame", Busy: busy}, began)
	snap, _ := tracker.Observe(panewatch.Frame{Hash: "later-frame", Busy: busy}, waveTestClock.Add(-20*time.Second))
	if err := panewatch.Write(r.runDir, snap); err != nil {
		t.Fatal(err)
	}
}

func (r *seatRig) deliverAt(t *testing.T, writtenAt time.Time) {
	t.Helper()
	path := filepath.Join(r.runDir, "test-report.md")
	if err := os.WriteFile(path, []byte("# TDD Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, writtenAt, writtenAt); err != nil {
		t.Fatal(err)
	}
}

func TestWaveStatus_SeatHealth_ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound(t *testing.T) {
	dispatchBegan := waveTestClock.Add(-15 * time.Minute)
	earlierRound := dispatchBegan.Add(-30 * time.Minute)
	cases := []struct {
		name  string
		busy  bool
		alive bool
		want  string
	}{
		{"a seat at its prompt", false, true, "blocked"},
		{"a seat whose session is gone", true, false, "zombie"},
		{"a busy seat with fresh output", true, true, "working"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newSeatRig(t)
			rig.openRun(t)
			rig.dispatchBeganAt(t, dispatchBegan, tc.busy)
			rig.deliverAt(t, earlierRound)
			if tc.alive {
				rig.sessionAlive()
			}

			wantOneSeat(t, rig.statusJSON(t), tc.want)
		})
	}
}

func TestWaveStatus_SeatHealth_ADeliverableWrittenByThisRoundReadsIdle(t *testing.T) {
	dispatchBegan := waveTestClock.Add(-15 * time.Minute)
	for name, alive := range map[string]bool{"a listed session": true, "a gone session": false} {
		t.Run(name, func(t *testing.T) {
			rig := newSeatRig(t)
			rig.openRun(t)
			rig.dispatchBeganAt(t, dispatchBegan, false)
			rig.deliverAt(t, dispatchBegan.Add(5*time.Minute))
			if alive {
				rig.sessionAlive()
			}

			wantOneSeat(t, rig.statusJSON(t), "idle")
		})
	}
}

func TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.tmuxErr = errors.New("signal: killed: context deadline exceeded")

	doc := rig.statusDoc(t)

	wantOneSeat(t, doc.Seats, "blocked")
	var named []string
	for _, w := range doc.Warnings {
		if strings.Contains(w, seatSocket) && strings.Contains(w, "context deadline exceeded") {
			named = append(named, w)
		}
	}
	if len(named) != 1 {
		t.Errorf("warnings = %q, want exactly one naming socket %s and the cause of the list failure", doc.Warnings, seatSocket)
	}
	if text := rig.statusText(t); !strings.Contains(text, "context deadline exceeded") {
		t.Errorf("text status hides the list failure from the operator:\n%s", text)
	}
}

func TestWaveStatus_SeatHealth_AGoneServerIsAZombieNotAWarning(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, true, 20*time.Second)
	rig.tmuxErr = errors.New("no server running on /private/tmp/tmux-501/" + seatSocket)

	doc := rig.statusDoc(t)

	wantOneSeat(t, doc.Seats, "zombie")
	for _, w := range doc.Warnings {
		if strings.Contains(w, seatSocket) {
			t.Errorf("warning %q reports a gone server, which is the zombie evidence itself", w)
		}
	}
}

func TestWaveStatus_SeatHealth_AHealthyListingRaisesNoWarning(t *testing.T) {
	rig := newSeatRig(t)
	rig.openRun(t)
	rig.pane(t, false, 20*time.Second)
	rig.sessionAlive()

	doc := rig.statusDoc(t)

	wantOneSeat(t, doc.Seats, "blocked")
	for _, w := range doc.Warnings {
		if strings.Contains(w, seatSocket) {
			t.Errorf("warning %q on a clean listing", w)
		}
	}
}

func TestBridgeTmuxListing_AFailureWithNoStderrNamesNoEmptyReason(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := bridgeSessionsTmux(context.Background(), "evolve-bridge-p1")

	if err == nil {
		t.Fatal("a missing tmux binary must be an error")
	}
	if msg := err.Error(); strings.HasSuffix(strings.TrimSpace(msg), ":") {
		t.Errorf("error %q ends in a separator with no reason after it", msg)
	}
}

func inACLocale(t *testing.T) {
	t.Helper()
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "C")
}

func TestWaveStatus_SeatHealth_ARealTmuxSessionReadsBlockedUnderACLocale(t *testing.T) {
	seat := newRealTmuxSeat(t)
	inACLocale(t)
	seat.tmux(t, "new-session", "-d", "-s", seat.session, "sleep 600")
	seat.writePane(t, false)

	seat.wantSeat(t, "blocked")
}

func TestBridgeSessions_ARealTmuxSessionListsAsUnwatchedUnderACLocale(t *testing.T) {
	seat := newRealTmuxSeat(t)
	inACLocale(t)
	seat.tmux(t, "new-session", "-d", "-s", seat.session, "sleep 600")
	bridgeLiveSockets = func() []string { return []string{seat.socket} }

	rows, _ := runSessionsJSON(t, t.TempDir())

	if len(rows) != 1 || rows[0].Session != seat.session || rows[0].State != "unwatched" {
		t.Errorf("rows = %+v, want the one live session %s listed as unwatched", rows, seat.session)
	}
}
