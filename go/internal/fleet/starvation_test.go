package fleet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// starvation_test.go — non-acs regression coverage for the recovered L3
// work-supply-starvation observer (starvation.go). The ACS predicates in
// go/acs/cycle544 are the cycle-scoped gate; these tests are the PERMANENT,
// package-local coverage of the observer's public contract and directly satisfy
// AC-4 (cmd_loop.go's `case ran:` side effect covered OUTSIDE package main) and
// AC-5 (quota-shrunk anti-no-op) via package-fleet unit tests — the coverage the
// recovered starvation.go needs and that the cycle-543 diff never provided.

// TestStarvation_QuotaShrunkWaveNeverStarves — AC-5. A quota/capacity shrink
// (waveCfg.Count < fleetCfg.Count ⇒ QuotaShrunk=true) that leaves DesiredLanes
// unrealized is NEVER work-supply starvation. Repeated shrunk waves must never
// advance the streak nor fire. Anti-no-op: an impl that keys off
// RealizedLanes < DesiredLanes alone fails.
func TestStarvation_QuotaShrunkWaveNeverStarves(t *testing.T) {
	shrunk := WaveObservation{DesiredLanes: 4, RealizedLanes: 1, QuotaShrunk: true}
	if shrunk.Starved() {
		t.Fatalf("quota-shrunk wave (DesiredLanes=%d RealizedLanes=%d) reported Starved()=true — a capacity shrink is never work-supply starvation", shrunk.DesiredLanes, shrunk.RealizedLanes)
	}
	var tr StarvationTracker
	for wave := 0; wave < 6; wave++ {
		if tr.Observe(shrunk, 3) {
			t.Fatalf("quota-shrunk wave %d fired starvation — QuotaShrunk must suppress the detector", wave)
		}
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak=%d after only quota-shrunk waves, want 0", tr.Streak())
	}
}

// TestStarvation_ObserveFiresAfterKAndWritesInboxTodo — AC-4. Drives the exact
// side effect cmd_loop.go's `case ran:` arm performs (observe→build→WriteTo),
// here outside package main. DesiredLanes/RealizedLanes name the operator-
// asserted vs realized lane counts. Asserts the K-th consecutive starved wave
// fires and persists one cause-stable inbox todo.
func TestStarvation_ObserveFiresAfterKAndWritesInboxTodo(t *testing.T) {
	const k = 3
	obs := WaveObservation{DesiredLanes: 3, RealizedLanes: 1, QuotaShrunk: false}
	if !obs.Starved() {
		t.Fatalf("WaveObservation{DesiredLanes:%d RealizedLanes:%d}.Starved()=false, want true", obs.DesiredLanes, obs.RealizedLanes)
	}
	var tr StarvationTracker
	for wave := 0; wave < k-1; wave++ {
		if tr.Observe(obs, k) {
			t.Fatalf("fired on starved wave %d, want no fire before K=%d", wave, k)
		}
	}
	if !tr.Observe(obs, k) {
		t.Fatalf("did not fire on the %d-th consecutive starved wave (K=%d)", k, k)
	}

	item := BuildStarvationItem(obs, k, 0.5, 544, "2026-07-06T00:00:00Z")
	if item.Weight < 0.9 {
		t.Fatalf("BuildStarvationItem weight=%v, want clamped up to 0.9 floor", item.Weight)
	}
	dir := t.TempDir()
	path, err := item.WriteTo(dir)
	if err != nil {
		t.Fatalf("WriteTo(%s): %v", dir, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written todo %s: %v", path, err)
	}
	var got StarvationItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("written todo not valid JSON: %v", err)
	}
	if got.ID != "fleet-work-supply-starvation" {
		t.Fatalf("todo id=%q, want cause-stable fleet-work-supply-starvation", got.ID)
	}
	if got.Source != "fleet-starvation-observer" {
		t.Fatalf("todo source=%q, want fleet-starvation-observer", got.Source)
	}
	if filepath.Base(path) != "fleet-work-supply-starvation.json" {
		t.Fatalf("filename=%q, want fleet-work-supply-starvation.json", filepath.Base(path))
	}
}

// TestStarvation_FireResetsStreakAndRecoveryResets — the streak spans waves,
// resets on a fire (next fire needs a fresh K), and resets on a recovered wave.
func TestStarvation_FireResetsStreakAndRecoveryResets(t *testing.T) {
	starved := WaveObservation{DesiredLanes: 2, RealizedLanes: 1}
	var tr StarvationTracker
	if tr.Observe(starved, 2) {
		t.Fatalf("fired on wave 1 with K=2")
	}
	if !tr.Observe(starved, 2) {
		t.Fatalf("did not fire on wave 2 with K=2")
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak=%d after fire, want 0", tr.Streak())
	}
	// Advance one starved wave (streak→1) then recover: streak must return to 0.
	tr.Observe(starved, 5)
	if got := tr.Streak(); got != 1 {
		t.Fatalf("streak=%d after one starved wave, want 1", got)
	}
	recovered := WaveObservation{DesiredLanes: 2, RealizedLanes: 2}
	if tr.Observe(recovered, 5) {
		t.Fatalf("recovered wave fired")
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak=%d after recovered wave, want 0", tr.Streak())
	}
}

// TestStarvation_KBelowOneClampsToOne — k<1 is treated as k=1, so a single
// starved wave fires immediately.
func TestStarvation_KBelowOneClampsToOne(t *testing.T) {
	var tr StarvationTracker
	starved := WaveObservation{DesiredLanes: 2, RealizedLanes: 0}
	if !tr.Observe(starved, 0) {
		t.Fatalf("k=0 should clamp to 1 and fire on the first starved wave")
	}
}

// TestStarvationItem_ValidateRejectsUnderweightAndMissingFields — a malformed
// self-injection must fail loud, never seed a silent no-op todo.
func TestStarvationItem_ValidateRejectsUnderweightAndMissingFields(t *testing.T) {
	good := BuildStarvationItem(WaveObservation{DesiredLanes: 3, RealizedLanes: 1}, 3, 0.9, 1, "2026-07-06T00:00:00Z")
	if err := good.Validate(); err != nil {
		t.Fatalf("Validate() on a well-formed item: %v", err)
	}
	under := good
	under.Weight = 0.5
	if err := under.Validate(); err == nil {
		t.Fatalf("Validate() accepted an under-floor weight 0.5, want error")
	}
	missing := good
	missing.ID = ""
	if err := missing.Validate(); err == nil {
		t.Fatalf("Validate() accepted an item with an empty required field, want error")
	}
}
