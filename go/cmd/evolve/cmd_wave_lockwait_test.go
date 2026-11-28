package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func lockWaitEvent(pid int, seq uint64, cycle int, at time.Time, waitMS string) signalcenter.Event {
	return signalcenter.Event{
		PID: pid, Seq: seq, Cycle: cycle, TS: at.Format(time.RFC3339Nano),
		Code: signalcenter.CodeHostVerifyLockWait, Fields: map[string]string{"wait_ms": waitMS},
	}
}

func TestParseRunCycleDir_AcceptsOnlyACycleNumber(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		id int
		ok bool
	}{
		"cycle-7":  {7, true},
		"cycle-0":  {0, true},
		"cycle-":   {0, false},
		"cycle-x":  {0, false},
		"cycle--1": {0, false},
		"run-7":    {0, false},
	}
	for name, want := range cases {
		id, ok := parseRunCycleDir(name)
		if ok != want.ok || (ok && id != want.id) {
			t.Errorf("parseRunCycleDir(%q) = %d, %v, want %d, %v", name, id, ok, want.id, want.ok)
		}
	}
}

func TestReadRunLockWaitSignals_SkipsStrayFilesForeignFoldersAndCyclesAtTheFloor(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	at := hostBase
	writeRunWait(t, evolveDir, 3, at, 1000)
	writeRunWait(t, evolveDir, 9, at, 2000)
	if err := os.WriteFile(filepath.Join(evolveDir, "runs", "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs", "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs", "cycle-11"), 0o755); err != nil {
		t.Fatal(err)
	}
	var seen []string
	readRunLockWaitSignals(evolveDir, 3, func(e signalcenter.Event) { seen = append(seen, fmt.Sprint(e.Cycle, ":", e.Fields["wait_ms"])) })
	if len(seen) != 1 || seen[0] != "9:2000" {
		t.Errorf("visited = %v, want only 9:2000", seen)
	}
	readRunLockWaitSignals(filepath.Join(evolveDir, "missing"), -1, func(signalcenter.Event) { t.Error("visited an event of a missing runs folder") })
}

func TestSumWindowLockWait_IgnoresAMalformedWait(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	dir := filepath.Join(evolveDir, "runs", "cycle-4")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"schema_version":"signal/1.0","seq":1,"pid":1,"ts":%q,"cycle":4,"module":"host","origin":"o","kind":"loop.warning","code":"HOST_VERIFY_LOCK_WAIT","severity":"INFO","reason":"r","fields":{"wait_ms":"oops"}}`+"\n", hostBase.Format(time.RFC3339Nano))
	if err := os.WriteFile(filepath.Join(dir, "signals.ndjson"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRunWait(t, evolveDir, 5, hostBase, 3000)
	if got := sumWindowLockWait(evolveDir, time.Hour, hostBase.Add(time.Minute)); got != 3*time.Second {
		t.Errorf("sumWindowLockWait = %v, want 3s", got)
	}
}

func TestWaveLockWaitMS_CountsTheGlobalLogByCycleOrTimeAndOneWaitOnce(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	start := hostBase
	end := start.Add(4 * time.Hour)
	cur := wave.Record{Number: 1, RunID: "r1", CycleFloor: 5, StartedAt: start, EndedAt: &end}
	writeRunWait(t, evolveDir, 7, start.Add(time.Minute), 1000)
	global := []signalcenter.Event{
		lockWaitEvent(50007, 7, 7, start.Add(time.Minute), "1000"),
		lockWaitEvent(60001, 1, 8, start.Add(2*time.Minute), "2000"),
		lockWaitEvent(60002, 2, 5, start.Add(2*time.Minute), "4000"),
		lockWaitEvent(60003, 3, 0, start.Add(time.Hour), "8000"),
		lockWaitEvent(60004, 4, 0, start.Add(-time.Hour), "16000"),
		lockWaitEvent(60005, 5, 0, end.Add(time.Hour), "32000"),
		{PID: 60006, Seq: 6, TS: "not-a-time", Code: signalcenter.CodeHostVerifyLockWait, Fields: map[string]string{"wait_ms": "64000"}},
		lockWaitEvent(0, 0, 9, start.Add(time.Hour), "128"),
		lockWaitEvent(0, 0, 9, start.Add(time.Hour), "128"),
		lockWaitEvent(60007, 7, 9, start.Add(time.Hour), "-5"),
	}
	if got, want := waveLockWaitMS(evolveDir, cur, global), int64(1000+2000+8000+128+128); got != want {
		t.Errorf("waveLockWaitMS = %d, want %d", got, want)
	}
}
