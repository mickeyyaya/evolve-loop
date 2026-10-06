package panewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func identity() Snapshot {
	return Snapshot{Session: "evolve-bridge-agy-c7-router-pid1-n1-1", Socket: "evolve-bridge-p1", CLI: "agy-tmux", Agent: "router", Cycle: 7, Model: "Gemini 3.1 Pro (High)"}
}

func TestTracker_FirstObservationIsPublishedAsProgress(t *testing.T) {
	var tr *Tracker = NewTracker(identity())
	snap, changed := tr.Observe(Frame{Hash: "h1", Busy: true}, t0)
	if !changed {
		t.Fatal("the first frame must be published so a reader learns the phase runs on a pane")
	}
	if snap.ProgressAt != t0 || snap.UpdatedAt != t0 || snap.ProgressHash != "h1" || !snap.Busy {
		t.Errorf("first snapshot = %+v", snap)
	}
	if snap.Session != identity().Session || snap.CLI != "agy-tmux" || snap.Agent != "router" || snap.Cycle != 7 {
		t.Errorf("identity not carried: %+v", snap)
	}
}

func TestTracker_SameHashIsNotProgressAndNotRepublished(t *testing.T) {
	tr := NewTracker(identity())
	tr.Observe(Frame{Hash: "h1", Busy: true}, t0)
	snap, changed := tr.Observe(Frame{Hash: "h1", Busy: true}, t0.Add(10*time.Minute))
	if changed {
		t.Errorf("an unchanged frame was republished: %+v", snap)
	}
	if snap.ProgressAt != t0 {
		t.Errorf("progress_at advanced without a hash change: %v", snap.ProgressAt)
	}
}

func TestTracker_NewHashAdvancesProgress(t *testing.T) {
	tr := NewTracker(identity())
	tr.Observe(Frame{Hash: "h1", Busy: true}, t0)
	later := t0.Add(3 * time.Minute)
	snap, changed := tr.Observe(Frame{Hash: "h2", Busy: true}, later)
	if !changed || snap.ProgressAt != later || snap.ProgressHash != "h2" {
		t.Errorf("new transcript must advance progress: changed=%v snap=%+v", changed, snap)
	}
}

func TestTracker_BusyTokenOrLabelChangeRepublishesWithoutProgress(t *testing.T) {
	cases := []Frame{
		{Hash: "h1", Busy: false},
		{Hash: "h1", Busy: true, TokenLine: "▸ Thought for 14s, 1.5k tokens"},
		{Hash: "h1", Busy: true, ModelLabel: "Gemini 3.8 Flash · high"},
	}
	for _, next := range cases {
		tr := NewTracker(identity())
		tr.Observe(Frame{Hash: "h1", Busy: true}, t0)
		later := t0.Add(time.Minute)
		snap, changed := tr.Observe(next, later)
		if !changed || snap.UpdatedAt != later {
			t.Errorf("%+v: a state change must republish: changed=%v snap=%+v", next, changed, snap)
		}
		if snap.ProgressAt != t0 {
			t.Errorf("%+v: progress_at moved without new transcript: %v", next, snap.ProgressAt)
		}
	}
}

func TestWriteRead_RoundTripsTheSnapshot(t *testing.T) {
	ws := t.TempDir()
	want, _ := NewTracker(identity()).Observe(Frame{Hash: "h1", Busy: true, TokenLine: "▸ Thought for 2s, 264 tokens", ModelLabel: "Gemini 3.8 Flash · low"}, t0)
	if err := Write(ws, want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path(ws, "router")); err != nil {
		t.Fatalf("snapshot not at Path: %v", err)
	}
	got, ok, err := Read(ws, "router")
	if err != nil || !ok {
		t.Fatalf("Read = ok %v err %v", ok, err)
	}
	if got != want {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, want)
	}
}

func TestRead_MissingSnapshotIsNotAnError(t *testing.T) {
	_, ok, err := Read(t.TempDir(), "build")
	if ok || err != nil {
		t.Errorf("missing snapshot: ok=%v err=%v, want false, nil", ok, err)
	}
}

func TestRead_CorruptSnapshotIsAnError(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(Path(ws, "build"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Read(ws, "build"); ok || err == nil {
		t.Errorf("corrupt snapshot: ok=%v err=%v, want an error", ok, err)
	}
}

func TestReadAll_ListsEverySnapshotInTheWorkspace(t *testing.T) {
	ws := t.TempDir()
	for _, agent := range []string{"router", "build"} {
		s := identity()
		s.Agent = agent
		if err := Write(ws, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "build-observer-events.ndjson"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAll(ws)
	if err != nil || len(got) != 2 {
		t.Fatalf("ReadAll = %d snapshots, err %v; want 2", len(got), err)
	}
	if got[0].Agent != "build" || got[1].Agent != "router" {
		t.Errorf("ReadAll order = %s, %s; want build, router", got[0].Agent, got[1].Agent)
	}
}

func TestReadAll_SkipsACorruptSnapshotButReportsIt(t *testing.T) {
	ws := t.TempDir()
	if err := Write(ws, identity()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(ws, "build"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAll(ws)
	if err == nil || len(got) != 1 || got[0].Agent != "router" {
		t.Errorf("ReadAll = %+v, err %v; want the good snapshot and an error naming the bad one", got, err)
	}
}

func TestWrite_FailsLoudlyWhenTheWorkspaceIsNotADirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(file, identity()); err == nil {
		t.Error("Write into a regular file path must fail")
	}
}

func TestRemove_DeletesTheSnapshotAndToleratesItsAbsence(t *testing.T) {
	ws := t.TempDir()
	if err := Write(ws, identity()); err != nil {
		t.Fatal(err)
	}
	if err := Remove(ws, "router"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok, _ := Read(ws, "router"); ok {
		t.Error("snapshot still readable after Remove")
	}
	if err := Remove(ws, "router"); err != nil {
		t.Errorf("removing an absent snapshot must succeed: %v", err)
	}
}

func TestTracker_AgentNamesTheSnapshotFile(t *testing.T) {
	if got := NewTracker(identity()).Agent(); got != "router" {
		t.Errorf("Agent() = %q, want router", got)
	}
}

func TestSnapshot_WriterAliveNeedsALiveRecordedPID(t *testing.T) {
	alive := func(pid int) bool { return pid == 42 }
	if !(Snapshot{WriterPID: 42}).WriterAlive(alive) {
		t.Error("a snapshot written by a live pid is live")
	}
	if (Snapshot{WriterPID: 7}).WriterAlive(alive) {
		t.Error("a snapshot whose writer died is stale")
	}
	if (Snapshot{}).WriterAlive(func(int) bool { return true }) {
		t.Error("a snapshot with no writer pid cannot prove its writer lives")
	}
}

func TestReadLive_ADeadWritersSnapshotReadsAsAbsent(t *testing.T) {
	ws := t.TempDir()
	s := identity()
	s.WriterPID = 42
	if err := Write(ws, s); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := ReadLive(ws, "router", func(pid int) bool { return pid == 42 }); !ok || err != nil || got.Session != s.Session {
		t.Errorf("live writer: ok=%v err=%v snapshot=%+v", ok, err, got)
	}
	if _, ok, err := ReadLive(ws, "router", func(int) bool { return false }); ok || err != nil {
		t.Errorf("dead writer: ok=%v err=%v, want absent with no error", ok, err)
	}
	if err := os.WriteFile(Path(ws, "build"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := ReadLive(ws, "build", func(int) bool { return true }); ok || err == nil {
		t.Errorf("corrupt snapshot: ok=%v err=%v, want the read error", ok, err)
	}
}
