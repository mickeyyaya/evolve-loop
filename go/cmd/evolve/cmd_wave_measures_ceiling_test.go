package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestIntegrationVerdict_APendingMemberOfAnOpenWaveIsNotCalledPendingAtWaveEnd(t *testing.T) {
	t.Parallel()
	ev := cleanEvidence()
	ev.Record.Ended = false
	ev.Record.Members[7] = integration.MemberState{Status: integration.MemberPending}
	ev.Shipped = []int{6, 7}
	_, failed := integrationVerdict(ev)
	if strings.Contains(strings.Join(failed, "\n"), "still pending") {
		t.Fatalf("failed = %v, want only the missing end", failed)
	}
}

func measureRig(t *testing.T, floors []int, cycles []int) []waveRunMeasures {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	var records []wave.Record
	for i, f := range floors {
		records = append(records, wave.Record{Number: i + 1, RunID: "r", CycleFloor: f, StartedAt: at.Add(time.Duration(i) * time.Hour)})
	}
	for _, c := range cycles {
		in := landing.Intent{Cycle: c, CommitSHA: "1111111111111111111111111111111111111111", Status: landing.IntentComplete}
		if err := landing.WriteIntent(landing.IntentPath(evolveDir, c), in); err != nil {
			t.Fatal(err)
		}
	}
	runs, _ := measureWaveRuns(wavePlaneDirs{root: root, evolveDir: evolveDir}, records, nil, nil, false, at.Add(9*time.Hour))
	return runs
}

func TestMeasureWaveRuns_AWaveMeasuresOnlyTheCyclesBelowTheNextFloor(t *testing.T) {
	t.Parallel()
	runs := measureRig(t, []int{20, 40, 60}, []int{25, 45})
	if got := runs[0].Measures.Cost.Started; got != 1 {
		t.Fatalf("wave 1 counts %d cycles, want 1", got)
	}
}

func TestMeasureWaveRuns_AWaveWithAnEqualNextFloorMeasuresNoCycle(t *testing.T) {
	t.Parallel()
	runs := measureRig(t, []int{20, 20, 40}, []int{25, 30, 45})
	if got := runs[0].Measures.Cost.Started; got != 0 {
		t.Fatalf("wave 1 ran no cycle, and it counts %d", got)
	}
}

func TestWaveOfPool_APoolAboveALaterFloorIsNotGivenToAnEmptyWave(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t)
	r.seedWaves(t, 0, 20, 20, 40)
	if got := waveOfPool(r.root, 45); got != 4 {
		t.Fatalf("pool 45 reads as wave %d, want wave 4", got)
	}
	if got := waveOfPool(r.root, 25); got != 3 {
		t.Fatalf("pool 25 reads as wave %d, want wave 3", got)
	}
}
