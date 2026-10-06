package loopwave

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTally_CountsEachLaneOnceAsOkDeferredOrFailed(t *testing.T) {
	results := []fleet.Result{{ExitCode: fleet.ExitDeferred}, {ExitCode: 1}, {}, {Err: errors.New("spawn"), ExitCode: fleet.ExitDeferred}, {ExitCode: 2}}
	got := Tally(results)
	if got != (LaneTally{OK: 1, Deferred: 1, Failed: 3}) || got.Total() != len(results) {
		t.Errorf("Tally = %+v (total %d), want {OK:1 Deferred:1 Failed:3} over %d lanes", got, got.Total(), len(results))
	}
	if Tally(nil) != (LaneTally{}) {
		t.Error("no results, no lanes")
	}
}

func TestWaveSummary_NamesDeferredLanesOnlyWhenThereAreSome(t *testing.T) {
	reason, fields := WaveSummary(65, LaneTally{OK: 1, Failed: 1})
	if reason != "wave 65: 1/2 lanes ok" || fields["lanes_ok"] != "1" || fields["lanes"] != "2" || len(fields) != 2 {
		t.Errorf("no deferral keeps the line and fields unchanged: %q %v", reason, fields)
	}
	reason, fields = WaveSummary(65, LaneTally{OK: 1, Deferred: 1})
	if reason != "wave 65: 1/2 lanes ok, 1 deferred" || fields["lanes_deferred"] != "1" {
		t.Errorf("a deferred lane is named apart from the failed ones: %q %v", reason, fields)
	}
	if got := (LaneTally{Deferred: 2}).DeferredSuffix(); got != ", 2 deferred" {
		t.Errorf("DeferredSuffix = %q", got)
	}
}

type deferringLauncher struct{}

func (deferringLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	out := make([]fleet.Result, len(specs))
	for i := range specs {
		out[i] = fleet.Result{Index: i, ExitCode: fleet.ExitDeferred}
	}
	return out
}

func TestRepairMinWidth_ADeferredRepairLaneIsNamedDeferred(t *testing.T) {
	h := newHarness(t)
	h.e.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 0}, request(1, waveFC(1), pass, floorsPlan, deferringLauncher{}))
	want := "wave 1: min-width repair dispatched 0/1 isolated lane (fleet.count=2 shrank to 0), 1 deferred"
	if ev := h.only(t, CodeMinWidthRepair); ev.Reason != want {
		t.Errorf("reason = %q, want %q: a deferred repair lane is neither ok nor silently dropped", ev.Reason, want)
	}
}
