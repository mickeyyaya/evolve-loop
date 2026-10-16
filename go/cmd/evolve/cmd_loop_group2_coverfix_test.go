package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRenderRecurrenceReport_OmitsGenericPatterns(t *testing.T) {
	t.Parallel()
	led := recurrence.NewLedger()
	led.Entries["noise"] = &recurrence.Entry{Pattern: "noise", Count: 9, Generic: true}
	led.Entries["real"] = &recurrence.Entry{Pattern: "real", Count: 3}

	var buf bytes.Buffer
	renderRecurrenceReport(&buf, led)

	out := buf.String()
	if strings.Contains(out, "noise") {
		t.Fatalf("generic pattern must be de-noised out of the operator report:\n%s", out)
	}
	if !strings.Contains(out, "real") {
		t.Fatalf("non-generic pattern missing:\n%s", out)
	}
}

func TestRenderRecurrenceReport_EmptyLedgerSaysSoWithoutAHeader(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	renderRecurrenceReport(&buf, recurrence.NewLedger())

	if got := buf.String(); got != "recurrence: ledger is empty\n" {
		t.Fatalf("output = %q", got)
	}
}

func writeRunLease(t *testing.T, runDir string, heartbeat time.Time) {
	t.Helper()
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: 1}, heartbeat); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultBootRefreshFleetLane_FreshLeaseInAnyRunDirIsActive(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	runs := filepath.Join(evolveDir, "runs")
	writeRunLease(t, filepath.Join(runs, "cycle-1"), time.Now().Add(-2*runlease.DefaultTTL))
	writeRunLease(t, filepath.Join(runs, "cycle-2"), time.Now())
	if err := os.WriteFile(filepath.Join(runs, "stray-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	active, err := defaultBootRefreshFleetLane(loopConfig{EvolveDir: evolveDir})

	if err != nil || !active {
		t.Fatalf("active=%v err=%v, want true,nil", active, err)
	}
}

func TestDefaultBootRefreshFleetLane_OnlyStaleOrMissingLeasesIsInactive(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	runs := filepath.Join(evolveDir, "runs")
	writeRunLease(t, filepath.Join(runs, "cycle-1"), time.Now().Add(-2*runlease.DefaultTTL))
	if err := os.MkdirAll(filepath.Join(runs, "cycle-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, "stray-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	active, err := defaultBootRefreshFleetLane(loopConfig{EvolveDir: evolveDir})

	if err != nil || active {
		t.Fatalf("active=%v err=%v, want false,nil", active, err)
	}
}

func TestDefaultBootRefreshFleetLane_UnreadableRunsDirIsAnErrorNotInactive(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "runs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	active, err := defaultBootRefreshFleetLane(loopConfig{EvolveDir: evolveDir})

	if err == nil || active {
		t.Fatalf("active=%v err=%v, want false,error", active, err)
	}
}

func TestDefaultBootRefreshFleetLane_CorruptLeaseIsAnErrorNotInactive(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	runDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runlease.PathIn(runDir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	active, err := defaultBootRefreshFleetLane(loopConfig{EvolveDir: evolveDir})

	if err == nil || active {
		t.Fatalf("active=%v err=%v, want false,error", active, err)
	}
}

type readStateErrStorage struct{ fixtures.FakeStorage }

func (*readStateErrStorage) ReadState(context.Context) (core.State, error) {
	return core.State{}, os.ErrPermission
}

func TestReconcileStaleCycleState_UnreadableStateJSONLeavesRecordUntouched(t *testing.T) {
	t.Parallel()
	st := &readStateErrStorage{}
	st.CycleState = core.CycleState{CycleID: 3, Phase: "build"}
	var stderr bytes.Buffer

	reconcileStaleCycleState(context.Background(), st, t.TempDir(), time.Now(), &stderr)

	if got := st.CycleState.Phase; got != "build" {
		t.Fatalf("phase = %q, want build left untouched", got)
	}
	if !strings.Contains(stderr.String(), "state.json unreadable") {
		t.Fatalf("missing WARN:\n%s", stderr.String())
	}
}

func TestReconcileStaleCycleState_UnreadableLeaseNeverReconciles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runDir := cycleWorkspace(root, 3)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runlease.PathIn(runDir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &fixtures.FakeStorage{}
	st.CycleState = core.CycleState{CycleID: 3, Phase: "build"}
	st.State = core.State{LastCycleNumber: 3}
	var stderr bytes.Buffer

	reconcileStaleCycleState(context.Background(), st, root, time.Now(), &stderr)

	if got := st.CycleState.Phase; got != "build" {
		t.Fatalf("phase = %q, want build left untouched when liveness is unknown", got)
	}
	if !strings.Contains(stderr.String(), "owner liveness unknown") {
		t.Fatalf("missing WARN:\n%s", stderr.String())
	}
}
