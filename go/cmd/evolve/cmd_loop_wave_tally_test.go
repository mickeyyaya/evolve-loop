package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestCompleteWave_ADeferredLaneIsCountedDeferredNotOk(t *testing.T) {
	var stderr bytes.Buffer
	c := signalcenter.New()
	var waves []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindLoopWave {
			waves = append(waves, e)
		}
	})
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: t.TempDir(), EvolveDir: t.TempDir()},
		deps: orchDeps{Signals: c}, result: &loopResult{}, stdout: io.Discard, stderr: &stderr}
	results := []fleet.Result{{Index: 0}, {Index: 1, ExitCode: fleet.ExitDeferred}, {Index: 2, ExitCode: 2}}

	b.completeWave(65, policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 3}, results, 0, &fleet.StarvationTracker{})

	if !strings.Contains(stderr.String(), "[loop] wave 65: 1/3 lanes ok, 1 deferred\n") {
		t.Errorf("the wave line must count the deferred lane apart from the ok and failed ones:\n%s", stderr.String())
	}
	if len(waves) != 1 || waves[0].Fields["lanes_ok"] != "1" || waves[0].Fields["lanes_deferred"] != "1" || waves[0].Fields["lanes"] != "3" {
		t.Errorf("loop.wave fields = %+v, want lanes_ok=1 lanes_deferred=1 lanes=3", waves)
	}
}

func TestReportFleetResults_ADeferredLaneIsNotAFailure(t *testing.T) {
	var stderr bytes.Buffer
	code := reportFleetResults([]fleet.Result{{Index: 0}, {Index: 1, ExitCode: fleet.ExitDeferred}}, &stderr)

	if code != 0 {
		t.Errorf("exit %d, want 0: a deferral released its item unworked, nothing failed", code)
	}
	for _, want := range []string{"[fleet] cycle 1: deferred (exit=5", "[fleet] 1/2 cycles ok, 1 deferred\n"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if code := reportFleetResults([]fleet.Result{{Index: 0, ExitCode: 2}}, io.Discard); code != 1 {
		t.Errorf("a FAIL lane exits %d, want 1", code)
	}
}

func TestSoakLaunchesOK_ADeferredLaunchIsNotAFailure(t *testing.T) {
	var stderr bytes.Buffer
	ok := soakLaunchesOK([]fleet.Result{{}, {ExitCode: fleet.ExitDeferred}}, 2, &stderr)

	if !ok {
		t.Errorf("one clean launch and one deferral must pass the launch check")
	}
	if !strings.Contains(stderr.String(), "evolve fleet soak: 0/2 launches failed, 1 deferred\n") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if soakLaunchesOK([]fleet.Result{{}, {ExitCode: 1}}, 2, io.Discard) {
		t.Error("a failed launch fails the soak")
	}
}

func TestSoakLaunchesOK_ASoakWhereEveryLaunchDeferredIsInconclusive(t *testing.T) {
	var stderr bytes.Buffer
	ok := soakLaunchesOK([]fleet.Result{{ExitCode: fleet.ExitDeferred}, {ExitCode: fleet.ExitDeferred}}, 2, &stderr)

	if ok {
		t.Errorf("no launch ran, so the soak proved nothing and must not pass")
	}
	if !strings.Contains(stderr.String(), "inconclusive") {
		t.Errorf("stderr must say the soak is inconclusive: %q", stderr.String())
	}
}
