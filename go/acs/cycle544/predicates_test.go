//go:build acs

// Package cycle544 materialises the cycle-544 acceptance criteria for the single
// triage-committed (`## top_n`) task: recover-ship-fleet-starvation-observer.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane (blocker-solo,
//	Principle 5): recover-ship-fleet-starvation-observer. Every `## deferred`
//	item (report-size-contract-slice1, coverage-ssot-cli-and-gate-wiring, and the
//	rest of the out-of-scope backlog) gets ZERO predicates here — in particular
//	the recovered ciparity.CoverageTestArgs SSOT is deferred Task 3's surface and
//	is deliberately NOT bound by a cycle-544 predicate.
//
// FEATURE CONTEXT
//
//	Cycles 542 and 543 both built the L3 leg of the fleet-concurrency-respect
//	architecture — a work-supply-starvation observer that self-files ONE weighted
//	inbox todo after K consecutive waves realize fewer lanes than configured for a
//	reason OTHER than a quota/capacity shrink — and both FAILed to ship:
//	  - 542: landed the logic in a NEW leaf package internal/fleethealth, which
//	    escaped the apicover completeness gate (TestApicoverEnforce_* went RED).
//	  - 543: correctly moved it into internal/fleet but shipped a NON-HERMETIC
//	    ACS predicate (TestC543_008) that re-ran the whole real-git ship
//	    integration suite as a side effect and t.Fatalf'd on that foreign suite's
//	    exit code; it flaked red under contention and the audit kernel's
//	    Classify() correctly overrode the narrated PASS to FAIL (red_count>0).
//	This cycle recovers the additive, twice-audited diff and replaces the
//	non-hermetic predicate with the hermetic, in-process contract tests below.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real fleet.StarvationTracker / WaveObservation / BuildStarvationItem
// / WriteTo functions and asserts on the returned value, streak, or written
// artifact. None shells out to a foreign heavyweight suite (that is exactly the
// cycle-543 defect C544_003 guards against). The two file/dir checks (C544_002
// dir-absence, C544_003 self-hermeticity) carry an explicit config-check waiver.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C544_001 StarvationTracker fires on the K-th consecutive starved
//     wave, resets after firing, and a recovered wave resets the streak.
//   - Negative : C544_005 a QUOTA-SHRUNK under-utilized wave is NEVER starvation,
//     no matter how many consecutive such waves occur (the strongest anti-no-op:
//     a naive realized<desired impl that ignores QuotaShrunk FAILS here).
//   - E2E      : C544_004 the `case ran:` side effect — observe→build→WriteTo —
//     writes exactly ONE cause-stable inbox todo, weight clamped up to the floor.
//   - Regression (structural): C544_002 the observer stays inside the enforced
//     internal/fleet package, no internal/fleethealth leaf (cycle-542 anti-regression).
//   - Regression (meta): C544_003 no cycle-544 predicate shells a foreign real-git
//     integration suite (cycle-543 anti-regression).
//
// AC-6 (touched packages build/vet/race clean, repo-wide CI parity) and AC-7
// (guardcmd/opscmd coverage >=80%) are dispositioned manual+checklist (Auditor),
// NOT predicates: a whole-module `go build ./...` / `go vet` / `go test -race` /
// `go test -cover` nested inside an ACS `go test` is the very heavyweight-suite-
// in-a-predicate smell this cycle exists to eliminate, and those checks are the
// audit CI-parity gate's (ADR-0069) and coverage-gate persona's job. See
// test-report.md's Coverage Map + Handoff checklist for the exact commands.
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

// C544_001 — AC-1 (positive core). The recovered observer lands as
// fleet.StarvationTracker and encodes the streak contract: with K=2 it does NOT
// fire on the first starved wave, DOES fire on the 2nd consecutive one, resets
// its streak to 0 on firing (so the next fire needs a fresh K, not K+1), and a
// recovered (non-starved) wave resets the streak. Exercises the SUT directly.
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
	// A single starved wave then a recovered wave must reset the streak, not fire.
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

// C544_002 — AC-2 (structural anti-regression, config-check). The observer must
// extend the already-apicover-enforced internal/fleet package rather than a new
// leaf: cycle 542 built the identical logic as internal/fleethealth and the
// apicover completeness gate correctly FAILed it. Load-bearing assertions:
// (1) no internal/fleethealth directory exists, and (2) the observer symbol is
// reachable FROM package fleet (the compile-time fleet.StarvationTracker{}
// reference below proves placement — a fleethealth-housed impl would not
// satisfy this import). This is a package-placement invariant, not production
// behaviour, hence the config-check waiver.
//
// acs-predicate: config-check
func TestC544_002_ObserverInEnforcedFleetPkg_NoNewLeaf(t *testing.T) {
	// Compile-time proof the observer lives in package fleet (not a leaf pkg).
	var _ = fleet.StarvationTracker{}
	root := acsassert.RepoRoot(t)
	leaf := filepath.Join(root, "go", "internal", "fleethealth")
	if _, err := os.Stat(leaf); err == nil {
		t.Fatalf("internal/fleethealth exists (%s) — the observer must extend the already-enforced internal/fleet package, not add an apicover leaf (cycle-542 regression)", leaf)
	}
	// The recovered source must be present inside the enforced package.
	src := filepath.Join(root, "go", "internal", "fleet", "starvation.go")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("internal/fleet/starvation.go absent (%v) — the observer was not recovered into the enforced package", err)
	}
}

// C544_003 — AC-3 (meta anti-regression, config-check). The cycle-543 ship
// failure was a predicate that shelled out to a FOREIGN real-git integration
// suite and t.Fatalf'd on its exit code (non-hermetic; flaked under contention).
// This guard reads THIS predicate file and asserts it never references that
// foreign suite path nor builds a nested coverage-profile go-test invocation.
// Both needles are reconstructed from fragments so the guard never matches its
// OWN source text. This is an invariant on the test artifact itself (not
// production behaviour), hence the config-check waiver.
//
// acs-predicate: config-check
func TestC544_003_NoPredicateShellsForeignIntegrationSuite(t *testing.T) {
	root := acsassert.RepoRoot(t)
	self := filepath.Join(root, "go", "acs", "cycle544", "predicates_test.go")
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("read own predicate file %s: %v", self, err)
	}
	src := string(raw)
	// Reconstructed from fragments so this needle does not appear verbatim here.
	foreignShipSuite := "phases/" + "ship"
	if strings.Contains(src, foreignShipSuite) {
		t.Fatalf("cycle-544 predicate references the foreign real-git %s integration suite — reintroduces the cycle-543 non-hermetic predicate defect (D1)", foreignShipSuite)
	}
	nestedCoverProfile := "cover" + "profile"
	if strings.Contains(src, nestedCoverProfile) {
		t.Fatalf("cycle-544 predicate builds a nested -%s go-test invocation — ACS predicates must assert their SSOT hermetically in-process, never shell a heavyweight suite", nestedCoverProfile)
	}
}

// C544_004 — AC-4 (end-to-end side effect). Reproduces cmd_loop.go's `case ran:`
// starvation-observe path entirely in-process against a temp dir: K consecutive
// starved waves fire the tracker, BuildStarvationItem constructs the todo with
// its weight clamped UP to the floor (a below-floor caller weight must not slip
// through), and WriteTo persists exactly one cause-stable inbox JSON whose id and
// source name the starvation cause. This is the production self-file behaviour
// that lived unreachable in package main — now covered hermetically.
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
	// A below-floor caller weight (0.5) must be clamped up, never persisted as-is.
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
	// Idempotent-by-cause: the filename derives from the stable id.
	if base := filepath.Base(path); base != "fleet-work-supply-starvation.json" {
		t.Fatalf("inbox filename = %q, want cause-stable %q", base, "fleet-work-supply-starvation.json")
	}
}

// C544_005 — AC-5 (negative / anti-no-op). A quota/capacity shrink that reduces
// realized lanes below configured is NEVER work-supply starvation, no matter how
// many consecutive such waves occur — QuotaShrunk gates the whole detector. An
// implementation that fires on realized<desired while ignoring QuotaShrunk (a
// no-op that "detects starvation" by counting under-utilization) FAILS here.
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
