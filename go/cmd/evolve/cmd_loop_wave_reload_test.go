package main

// cmd_loop_wave_reload_test.go — TDD contract for cycle 739's sole committed
// top_n task fleet-config-hot-reload-wave-boundary (inbox item
// 2026-07-12T12-25-00Z, weight 0.92).
//
// Live incident 2026-07-12 (~20:17): the operator committed fleet.min_lanes
// 3->10 mid-batch (sanctioned ship a33ffd6a), but wave dispatch still shrank
// "wave count 10 -> 9 (min 3)" — RunLoop resolves the fleet policy block ONCE
// per batch (cmd_loop.go: "resolved once per batch") and every subsequent wave
// consumes the stale snapshot. The control-plane preflight already re-validates
// policy.json CLEANLINESS per wave; these tests pin the missing half: dispatch
// must consume the committed VALUES per wave too.
//
// Contract under test (the seam the batch loop must call at every wave
// boundary, before the quota/budget sizing):
//
//	reloadFleetConfigAtWaveBoundary(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig
//
//   - re-resolves the fleet block from the committed .evolve/policy.json via
//     the SAME loader semantics as batch start (loadFleetConfig);
//   - logs "[loop] fleet config reloaded: count=N min_lanes=M" to warn ONLY
//     when the resolved count/min_lanes changed vs prev;
//   - on an unreadable/malformed policy.json it HOLDS prev (the operator's
//     width commitment) and WARNs, instead of silently collapsing to the
//     Count=1 defaults the batch-start loader degrades to.
//
// DO NOT MODIFY THESE TESTS (builder contract) — implement the seam and its
// batch-loop wiring to make them pass.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// writeFleetPolicy writes .evolve-style policy.json with a fleet block into
// dir and returns dir (the evolveDir the loaders take).
func writeFleetPolicy(t *testing.T, dir, fleetJSON string) string {
	t.Helper()
	doc := `{"fleet":` + fleetJSON + `}`
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}
	return dir
}

// TestFleetDispatch_ReloadsMinLanesAtWaveBoundary is the incident twin: the
// operator commits min_lanes 3->10 between waves; the next wave boundary must
// resolve the NEW floor, and that floor must actually hold width under a
// quota bench (the exact "wave count 10 -> 9 (min 3)" shape that motivated
// the fix — with min_lanes=10 the bench is absorbed and width holds at 10).
func TestFleetDispatch_ReloadsMinLanesAtWaveBoundary(t *testing.T) {
	dir := writeFleetPolicy(t, t.TempDir(), `{"count":10,"min_lanes":3,"plan_source":"triage"}`)
	prev := loadFleetConfig(dir)
	if prev.Count != 10 || prev.MinLanes != 3 {
		t.Fatalf("fixture sanity: batch-start snapshot = count=%d min_lanes=%d, want 10/3", prev.Count, prev.MinLanes)
	}

	// Operator commits the width directive mid-batch.
	writeFleetPolicy(t, dir, `{"count":10,"min_lanes":10,"plan_source":"triage"}`)

	var warn bytes.Buffer
	got := reloadFleetConfigAtWaveBoundary(dir, prev, &warn)
	if got.MinLanes != 10 {
		t.Errorf("wave boundary kept stale min_lanes=%d, want 10 (committed change must take effect without a supervisor bounce)", got.MinLanes)
	}
	if got.Count != 10 {
		t.Errorf("count = %d, want 10 (unchanged by the min_lanes edit)", got.Count)
	}
	if !strings.Contains(warn.String(), "fleet config reloaded: count=10 min_lanes=10") {
		t.Errorf("changed values must log \"[loop] fleet config reloaded: count=10 min_lanes=10\"; got:\n%s", warn.String())
	}
	// Behavioral closure of the incident: under one benched family the NEW
	// floor absorbs the shrink — width holds at 10 instead of dropping to 9.
	if eff := fleet.QuotaAwareCount(got.Count, map[string]string{"codex": "rate_limit"}, got.MinLanes, io.Discard); eff != 10 {
		t.Errorf("QuotaAwareCount with reloaded floor = %d, want 10 (min_lanes=10 must absorb the bench)", eff)
	}
}

// TestFleetDispatch_ReloadsCountAtWaveBoundary pins the count half of the
// reload in both directions: widening (3->5) takes effect at the next wave,
// and narrowing to 1 takes effect too (dispatch drops to the sequential gate
// without requiring a lane-killing bounce).
func TestFleetDispatch_ReloadsCountAtWaveBoundary(t *testing.T) {
	dir := writeFleetPolicy(t, t.TempDir(), `{"count":3,"min_lanes":2,"plan_source":"triage"}`)
	prev := loadFleetConfig(dir)
	if prev.Count != 3 {
		t.Fatalf("fixture sanity: batch-start count = %d, want 3", prev.Count)
	}

	cases := []struct {
		name      string
		fleetJSON string
		wantCount int
		wantWave  bool
		wantLog   string
	}{
		{"widen-3-to-5", `{"count":5,"min_lanes":2,"plan_source":"triage"}`, 5, true, "fleet config reloaded: count=5 min_lanes=2"},
		{"narrow-to-1-exits-wave-path", `{"count":1,"plan_source":"triage"}`, 1, false, "fleet config reloaded: count=1 min_lanes=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFleetPolicy(t, dir, tc.fleetJSON)
			var warn bytes.Buffer
			got := reloadFleetConfigAtWaveBoundary(dir, prev, &warn)
			if got.Count != tc.wantCount {
				t.Errorf("reloaded count = %d, want %d", got.Count, tc.wantCount)
			}
			if shouldRunWave(got) != tc.wantWave {
				t.Errorf("shouldRunWave(reloaded) = %v, want %v", shouldRunWave(got), tc.wantWave)
			}
			if !strings.Contains(warn.String(), tc.wantLog) {
				t.Errorf("want reload log %q; got:\n%s", tc.wantLog, warn.String())
			}
		})
	}
}

// TestFleetDispatch_UnchangedPolicyByteIdenticalDispatch is the regression
// pin: when nothing changed on disk, the wave boundary must produce a config
// deep-equal to the batch-start snapshot AND stay silent — no spurious
// "reloaded" log noise on the steady-state path (one line per wave would be
// indistinguishable from real operator changes).
func TestFleetDispatch_UnchangedPolicyByteIdenticalDispatch(t *testing.T) {
	dir := writeFleetPolicy(t, t.TempDir(), `{"count":3,"min_lanes":2,"plan_source":"triage"}`)
	prev := loadFleetConfig(dir)

	var warn bytes.Buffer
	got := reloadFleetConfigAtWaveBoundary(dir, prev, &warn)
	if !reflect.DeepEqual(got, prev) {
		t.Errorf("unchanged policy.json must resolve an identical config:\nprev=%+v\ngot =%+v", prev, got)
	}
	if warn.Len() != 0 {
		t.Errorf("unchanged policy must log NOTHING; got:\n%s", warn.String())
	}
}

// TestFleetDispatch_MalformedPolicyAtWaveBoundaryHoldsWidth is the negative:
// a transiently unreadable/malformed policy.json at a wave boundary must NOT
// silently collapse the fleet to the Count=1 loader defaults (width is an
// operator commitment — [fleet_width_always_respected]). The seam holds the
// previous config and surfaces a WARN.
func TestFleetDispatch_MalformedPolicyAtWaveBoundaryHoldsWidth(t *testing.T) {
	dir := writeFleetPolicy(t, t.TempDir(), `{"count":3,"min_lanes":2,"plan_source":"triage"}`)
	prev := loadFleetConfig(dir)

	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("corrupt policy.json: %v", err)
	}
	var warn bytes.Buffer
	got := reloadFleetConfigAtWaveBoundary(dir, prev, &warn)
	if got.Count != prev.Count || got.MinLanes != prev.MinLanes {
		t.Errorf("malformed policy collapsed width: got count=%d min_lanes=%d, want held %d/%d",
			got.Count, got.MinLanes, prev.Count, prev.MinLanes)
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Errorf("malformed policy at wave boundary must WARN (never silent); got:\n%s", warn.String())
	}
}
