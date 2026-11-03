package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type failingResumeStarter struct{ recordingStarter }

func (failingResumeStarter) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return core.CycleResult{}, errors.New("resume bootstrap failed")
}

func TestNewLaneResume_TheComposedRecordRefusesACycleThatAnotherLaneHolds(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 7)
	ws := filepath.Dir(p.statePath)
	if err := runlease.Write(ws, runlease.Lease{RunID: "lane-b-run", OwnerPID: os.Getpid() + 1}, time.Now()); err != nil {
		t.Fatal(err)
	}

	_, admitted, err := newLaneResume(p.root, p.evolveDir, []string{"item-a"}, nil, nil, nil, &bytes.Buffer{}).record(p.statePath, true)

	var held *core.ResumeHeldError
	if admitted || !errors.As(err, &held) || held.Holder.RunID != "lane-b-run" {
		t.Fatalf("record = (admitted %v, err %v), want a *core.ResumeHeldError naming lane-b-run", admitted, err)
	}
}

func TestLaneResume_AnAdoptionThatFailsBeforeTheHeartbeatReleasesTheAdmissionLease(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		setup   func(t *testing.T, p pausedLane, l *laneResume)
		starter func(*startedCycle) cycleStarter
		wantErr bool
	}{
		"a failed reclaim": {func(_ *testing.T, _ pausedLane, l *laneResume) {
			l.claim = func(string, int, []string) error { return errors.New("claim failed") }
		}, func(g *startedCycle) cycleStarter { return recordingStarter{g} }, false},
		"a resume that fails before the heartbeat": {func(*testing.T, pausedLane, *laneResume) {}, func(g *startedCycle) cycleStarter {
			return failingResumeStarter{recordingStarter{g}}
		}, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := seedPausedLane(t, "item-a", 7)
			lane := p.lane(&bytes.Buffer{}, nil, "item-a")
			tc.setup(t, p, &lane)
			var got startedCycle

			_, err := lane.run(context.Background(), tc.starter(&got), laneRequest())

			if (err != nil) != tc.wantErr {
				t.Fatalf("run err = %v, want error %v", err, tc.wantErr)
			}
			if l, ok, rerr := runlease.Read(filepath.Dir(p.statePath)); rerr != nil || ok {
				t.Errorf("lease after the failed adoption = (%+v, %v, %v), want none: the paused cycle must not wait a full TTL", l, ok, rerr)
			}
		})
	}
}

func TestLaneResume_AnUnreleasableAdmissionLeaseIsReported(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 7)
	var stderr bytes.Buffer
	lane := p.lane(&stderr, nil, "item-a")
	ws := filepath.Dir(p.statePath)
	lane.claim = func(string, int, []string) error {
		return os.WriteFile(runlease.PathIn(ws), []byte("{not json"), 0o644)
	}
	var got startedCycle

	if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
		t.Fatalf("run err %v, resumed %+v; want a fresh cycle", err, got.resumed)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("cannot release the admission lease")) {
		t.Errorf("stderr = %q, want the release failure reported", stderr.String())
	}
}
