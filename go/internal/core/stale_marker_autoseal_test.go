package core

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestMarkerShouldAutoseal_DeadOwner(t *testing.T) {
	if !markerShouldAutoseal(999999, true /*hasPID*/, func(int) bool { return false /*dead*/ }) {
		t.Error("a marker whose owner PID is dead must be auto-sealed")
	}
}

func TestMarkerShouldAutoseal_LiveOwnerUntouched(t *testing.T) {
	self := os.Getpid()
	alive := func(pid int) bool { return pid == self }
	if markerShouldAutoseal(self, true, alive) {
		t.Error("a marker owned by a live process must NOT be auto-sealed")
	}
}

func TestMarkerShouldAutoseal_MissingPidFailsSafe(t *testing.T) {
	if !markerShouldAutoseal(0, false /*hasPID*/, func(int) bool { return true /*would-be-alive, must be ignored*/ }) {
		t.Error("a marker that cannot assert liveness (missing/zero pid) must fail SAFE toward auto-seal")
	}
}

func TestAutosealStaleMarker_DeadOwnerSealsViaSealCycleAndClearsBlock(t *testing.T) {
	evolveDir := t.TempDir()
	workspace := sealFixture(t, evolveDir, 491)
	// A lease that would read as FRESH (heartbeat == opts.Now) yet whose owner
	// pid is dead — proves liveness is pid-based, not merely heartbeat-age based,
	// and that AutosealStaleMarker forces the seal over a "fresh" lease.
	frozen := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	if err := runlease.Write(workspace, runlease.Lease{RunID: "run-491", OwnerPID: 999999}, frozen); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	dead := func(int) bool { return false }

	led := &recordingLedger{}
	res, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if err != nil {
		t.Fatalf("AutosealStaleMarker: %v", err)
	}
	if !sealed {
		t.Fatal("dead-owner marker must be auto-sealed even when its lease heartbeat looks fresh")
	}
	if res.SealedCycleID != 491 {
		t.Errorf("must seal the stranded cycle 491; sealed %d", res.SealedCycleID)
	}
	if len(led.entries) != 1 {
		t.Errorf("auto-seal must REUSE SealCycle (exactly one ledger append); got %d — a bespoke seal path duplicates logic (AC5)", len(led.entries))
	}

	_, sealed2, err2 := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if sealed2 {
		t.Error("the marker must be cleared after seal; a second autoseal must be a no-op")
	}
	if !errors.Is(err2, ErrNothingToReset) {
		t.Errorf("after seal the marker is gone → ErrNothingToReset; got %v", err2)
	}
}
