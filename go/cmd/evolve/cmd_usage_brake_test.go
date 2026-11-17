package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	usageHoldStopReason = "loop_usage_hold"
	usageHoldPolicyJSON = `{"cli_health":{"usage_hold_percent":90}}`
	usageBrakeResetText = "Nov 20, 4am (UTC)"
)

var (
	usageBrakeResetsAhead = time.Date(2099, 11, 20, 4, 0, 0, 0, time.UTC)
	usageBrakeResetsPast  = time.Date(2020, 1, 1, 4, 0, 0, 0, time.UTC)
)

type usageBrakeRig struct {
	b         *loopBatchCoordinator
	evolveDir string
	center    *signalcenter.Center
	stderr    *bytes.Buffer
	preWave   *int
}

func newUsageBrakeRig(t *testing.T, policyJSON string) usageBrakeRig {
	t.Helper()
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(paths.LoopStopPath(evolveDir)); err == nil {
		t.Fatal("the fixture starts with the operator brake engaged; the usage hold must stop the loop with no console action")
	}
	u13StubRefresh(t, func() bool { return false })
	stderr := &bytes.Buffer{}
	center := signalcenter.New()
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: &bytes.Buffer{}, stderr: stderr}
	b.deps.Signals = center
	b.deps.Storage = &fixtures.FakeStorage{}
	preWave := 0
	b.preWave = func(int) (batchDecision, bool) { preWave++; return batchDecision{flow: batchProceed}, false }
	return usageBrakeRig{b: b, evolveDir: evolveDir, center: center, stderr: stderr, preWave: &preWave}
}

func (r usageBrakeRig) boundary() batchDecision {
	fc, bin := policy.FleetConfig{Count: 1}, ""
	return r.b.prepareIteration(1, &fc, &bin, 0)
}

func (r usageBrakeRig) recordWeek(t *testing.T, percent float64, resetsAt time.Time) {
	t.Helper()
	obs := usageprobe.Observation{CLI: "claude", ObservedAt: time.Now().UTC(), Windows: []quotastate.UsageWindow{{
		Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: percent,
		ResetsText: usageBrakeResetText, ResetsAt: &resetsAt, Family: "claude",
	}}}
	if err := usageprobe.RecordObservation(r.evolveDir, obs); err != nil {
		t.Fatal(err)
	}
}

func (r usageBrakeRig) usageHaltSignals() []signalcenter.Event {
	r.center.Flush()
	var halts []signalcenter.Event
	for _, e := range r.center.Recent() {
		if e.Kind == signalcenter.KindLoopHalt && e.Fields["stop_reason"] == usageHoldStopReason {
			halts = append(halts, e)
		}
	}
	return halts
}

func TestUsageBrake_AReadingAtTheLimitStopsTheLoopAtTheNextBoundaryWithNoConsolePresent(t *testing.T) {
	for name, percent := range map[string]float64{"past the limit": 97, "exactly at the limit": 90} {
		t.Run(name, func(t *testing.T) {
			rig := newUsageBrakeRig(t, usageHoldPolicyJSON)
			rig.recordWeek(t, percent, usageBrakeResetsAhead)

			d := rig.boundary()

			if d.flow != batchStopIterations || rig.b.result.StopReason != usageHoldStopReason {
				t.Fatalf("flow=%v stop=%q, want the iterations to stop with %q", d.flow, rig.b.result.StopReason, usageHoldStopReason)
			}
			if halts := rig.usageHaltSignals(); len(halts) != 1 {
				t.Errorf("loop.halt signals with stop_reason %s = %d, want exactly 1 so wave status shows the cause: %+v", usageHoldStopReason, len(halts), rig.center.Recent())
			}
			log := rig.stderr.String()
			if !strings.Contains(log, "90") {
				t.Errorf("the loop log %q does not name the limit 90", log)
			}
			if !strings.Contains(log, usageBrakeResetText) && !strings.Contains(log, "2099-11-20T04:00:00Z") {
				t.Errorf("the loop log %q does not name the reset", log)
			}
		})
	}
}

func TestUsageBrake_TheBoundaryProceedsWhenTheReadingIsUnderTheLimitTheBrakeIsOffOrTheWindowReset(t *testing.T) {
	cases := []struct {
		name     string
		policy   string
		percent  float64
		resetsAt time.Time
	}{
		{"just under the limit", usageHoldPolicyJSON, 89.9, usageBrakeResetsAhead},
		{"no hold percent in the policy", `{"cli_health":{}}`, 99, usageBrakeResetsAhead},
		{"no policy file at all", "", 99, usageBrakeResetsAhead},
		{"the window reset already", usageHoldPolicyJSON, 99, usageBrakeResetsPast},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newUsageBrakeRig(t, tc.policy)
			rig.recordWeek(t, tc.percent, tc.resetsAt)

			d := rig.boundary()

			if d.flow != batchProceed || *rig.preWave != 1 || rig.b.result.StopReason != "" {
				t.Errorf("flow=%v preWave=%d stop=%q, want the boundary to proceed (stderr=%s)", d.flow, *rig.preWave, rig.b.result.StopReason, rig.stderr.String())
			}
		})
	}
}

func TestUsageBrake_AnUnreadableReadingIsNotUnderTheLimitAndStopsTheLoopNamingTheFile(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, rig usageBrakeRig)
	}{
		{"the windows file is corrupt", func(t *testing.T, rig usageBrakeRig) {
			if err := os.WriteFile(usageprobe.WindowsPath(rig.evolveDir), []byte(`{"claude": {not json`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"the last probe recorded an error and no reading", func(t *testing.T, rig usageBrakeRig) {
			obs := usageprobe.Observation{CLI: "claude", ObservedAt: time.Now().UTC(), Error: "the usage pane did not render"}
			if err := usageprobe.RecordObservation(rig.evolveDir, obs); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newUsageBrakeRig(t, usageHoldPolicyJSON)
			tc.write(t, rig)

			d := rig.boundary()

			if d.flow != batchStopIterations || rig.b.result.StopReason != usageHoldStopReason {
				t.Fatalf("flow=%v stop=%q, want an unreadable reading to stop the loop with %q", d.flow, rig.b.result.StopReason, usageHoldStopReason)
			}
			log := rig.stderr.String()
			if !strings.Contains(strings.ToLower(log), "unreadable") || !strings.Contains(log, "usage-windows.json") {
				t.Errorf("the loop log %q must say that the reading is unreadable and name usage-windows.json, so the operator can act", log)
			}
		})
	}
}

func TestUsageBrake_AnUnreadableReadingDoesNotStopTheLoopWhenTheBrakeIsOff(t *testing.T) {
	rig := newUsageBrakeRig(t, "")
	if err := os.WriteFile(usageprobe.WindowsPath(rig.evolveDir), []byte(`{"claude": {not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := rig.boundary()

	if d.flow != batchProceed || *rig.preWave != 1 {
		t.Errorf("flow=%v preWave=%d, want the boundary to proceed when no hold percent is set", d.flow, *rig.preWave)
	}
}
