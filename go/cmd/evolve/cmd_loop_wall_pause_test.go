package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func pausedResult(cycle int, writeErr string, ends ...core.FamilyEnd) core.CycleResult {
	return core.CycleResult{Cycle: cycle, QuotaWall: ends, WallPhase: "audit", WallWriteErr: writeErr}
}

func loginEvents(got []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range got {
		if e.Kind == signalcenter.KindLoginRequired {
			out = append(out, e)
		}
	}
	return out
}

func TestCycleRunQuotaExit_RendersTheWallFromTheResultWhenNoCheckpointWasWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	c, got := recordedCenter()
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(pausedResult(2174, "disk full", loginEnd, quotaEnd), root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), c)

	events := loginEvents(*got)
	if code != fleet.ExitDeferred || len(events) != 1 {
		t.Fatalf("code=%d events=%+v", code, events)
	}
	want := "cycle 2174 phase audit: claude needs a login; agy is on quota, reset unknown. Impact: the cycle is paused; no claim was held; the checkpoint write failed: disk full. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."
	if events[0].Reason != want || !strings.Contains(stdout.String(), `"login-required"`) {
		t.Errorf("\n got %q\nwant %q\nstdout %s", events[0].Reason, want, stdout.String())
	}
}

func TestHandleCycleError_ASequentialWallNamesTheLoginFromTheResult(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := seedClaimedCycle(t, root)
	c, got := recordedCenter()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger(), Signals: c}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	d := b.handleCycleError(pausedResult(7, "", loginEnd), core.ErrAllFamiliesExhausted)

	events := loginEvents(*got)
	if d.exitCode != 5 || len(events) != 1 {
		t.Fatalf("exit=%d events=%+v", d.exitCode, events)
	}
	want := "cycle 7 phase audit: claude needs a login. Impact: the cycle is paused; claim kept for resume. Time alone does not clear this. Next: open claude and run /login. Then evolve loop --resume."
	if events[0].Reason != want {
		t.Errorf("\n got %q\nwant %q", events[0].Reason, want)
	}
}
