package main

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
	if eff := fleet.QuotaAwareCount(got.Count, map[string]string{"codex": "rate_limit"}, got.MinLanes, io.Discard); eff != 10 {
		t.Errorf("QuotaAwareCount with reloaded floor = %d, want 10 (min_lanes=10 must absorb the bench)", eff)
	}
}

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
