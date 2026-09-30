//go:build acs

package cycle544

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC544_001_StarvationTracker_FiresOnKthConsecutiveStarvedWave(t *testing.T) {
	starved := fleet.WaveObservation{DesiredLanes: 2, RealizedLanes: 1, QuotaShrunk: false}
	if !starved.Starved() {
		t.Fatalf("WaveObservation{realized<desired, !quotaShrunk}.Starved() = false, want true")
	}
	var tr fleet.StarvationTracker
	if tr.Observe(starved, 2) {
		t.Fatalf("fired on the 1st starved wave with K=2 — must wait for K consecutive")
	}
	if !tr.Observe(starved, 2) {
		t.Fatalf("did not fire on the 2nd consecutive starved wave with K=2")
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak = %d after firing, want 0 (a fire must reset the streak)", tr.Streak())
	}
	if tr.Observe(starved, 3) {
		t.Fatalf("fired on a lone starved wave with K=3")
	}
	recovered := fleet.WaveObservation{DesiredLanes: 2, RealizedLanes: 2, QuotaShrunk: false}
	if tr.Observe(recovered, 3) {
		t.Fatalf("a recovered (realized==desired) wave fired starvation")
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak = %d after a recovered wave, want 0 (recovery resets the streak)", tr.Streak())
	}
}

// acs-predicate: config-check
func TestC544_002_ObserverInEnforcedFleetPkg_NoNewLeaf(t *testing.T) {
	var _ = fleet.StarvationTracker{}
	root := acsassert.RepoRoot(t)
	leaf := filepath.Join(root, "go", "internal", "fleethealth")
	if _, err := os.Stat(leaf); err == nil {
		t.Fatalf("internal/fleethealth exists (%s) — the observer must extend the already-enforced internal/fleet package, not add an apicover leaf (cycle-542 regression)", leaf)
	}
	src := filepath.Join(root, "go", "internal", "fleet", "starvation.go")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("internal/fleet/starvation.go absent (%v) — the observer was not recovered into the enforced package", err)
	}
}

// acs-predicate: config-check
func TestC544_003_NoPredicateShellsForeignIntegrationSuite(t *testing.T) {
	root := acsassert.RepoRoot(t)
	self := filepath.Join(root, "go", "acs", "cycle544", "predicates_test.go")
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("read own predicate file %s: %v", self, err)
	}
	src := string(raw)
	foreignShipSuite := "phases/" + "ship"
	if strings.Contains(src, foreignShipSuite) {
		t.Fatalf("cycle-544 predicate references the foreign real-git %s integration suite — reintroduces the cycle-543 non-hermetic predicate defect (D1)", foreignShipSuite)
	}
	nestedCoverProfile := "cover" + "profile"
	if strings.Contains(src, nestedCoverProfile) {
		t.Fatalf("cycle-544 predicate builds a nested -%s go-test invocation — ACS predicates must assert their SSOT hermetically in-process, never shell a heavyweight suite", nestedCoverProfile)
	}
}

func TestC544_004_ObserveFiresBuildsAndWritesOneInboxTodo(t *testing.T) {
	const k = 3
	obs := fleet.WaveObservation{DesiredLanes: 3, RealizedLanes: 1, QuotaShrunk: false}
	var tr fleet.StarvationTracker
	fired := false
	for wave := 0; wave < k; wave++ {
		fired = tr.Observe(obs, k)
	}
	if !fired {
		t.Fatalf("tracker did not fire after %d consecutive starved waves with K=%d", k, k)
	}
	item := fleet.BuildStarvationItem(obs, k, 0.5, 544, "2026-07-06T00:00:00Z")
	if item.Weight < 0.9 {
		t.Fatalf("BuildStarvationItem weight = %v, want clamped up to the 0.9 floor (a starvation signal is never under-weighted)", item.Weight)
	}
	dir := t.TempDir()
	path, err := item.WriteTo(dir)
	if err != nil {
		t.Fatalf("WriteTo(%s): %v", dir, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("inbox todo not written at %s: %v", path, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written inbox todo: %v", err)
	}
	var got fleet.StarvationItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("written inbox todo is not valid JSON: %v", err)
	}
	if got.ID != "fleet-work-supply-starvation" {
		t.Fatalf("written todo id = %q, want the cause-stable %q (re-fires must overwrite one todo, not pile up)", got.ID, "fleet-work-supply-starvation")
	}
	if got.Source != "fleet-starvation-observer" {
		t.Fatalf("written todo source = %q, want %q", got.Source, "fleet-starvation-observer")
	}
	if got.Weight < 0.9 {
		t.Fatalf("persisted weight = %v, want >= 0.9 floor", got.Weight)
	}
	if base := filepath.Base(path); base != "fleet-work-supply-starvation.json" {
		t.Fatalf("inbox filename = %q, want cause-stable %q", base, "fleet-work-supply-starvation.json")
	}
}

func TestC544_005_QuotaShrunkWaveNeverStarves(t *testing.T) {
	shrunk := fleet.WaveObservation{DesiredLanes: 3, RealizedLanes: 1, QuotaShrunk: true}
	if shrunk.Starved() {
		t.Fatalf("a quota-shrunk wave reported Starved() = true — a capacity shrink is never work-supply starvation")
	}
	var tr fleet.StarvationTracker
	const k = 3
	for wave := 0; wave < k+3; wave++ {
		if tr.Observe(shrunk, k) {
			t.Fatalf("quota-shrunk wave %d fired starvation with K=%d — QuotaShrunk must suppress the detector entirely", wave, k)
		}
	}
	if tr.Streak() != 0 {
		t.Fatalf("streak = %d after only quota-shrunk waves, want 0 (they must never advance the streak)", tr.Streak())
	}
}
