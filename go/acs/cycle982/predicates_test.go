//go:build acs

// Package cycle982 materializes the cycle-982 acceptance criteria for the sole
// fleet lane this cycle is pinned to: salvage-prefix-queue-composer-core
// (goal 1f6d5bf8…, campaign merge-efficiency-2026-07). Per R9.3 no predicate
// here binds to any other lane's items — fleet_scope pins this lane to exactly
// this id and its three scout-derived, triage-committed (## top_n) tasks.
//
// Cycle-981 landed the prefix composer (go/internal/fleet/prefixqueue.go) and a
// PlanLanding *plan-emitting* seam, but left the core value INERT and unsafe:
//
//	T1 wire-resolveculprit-ship-landing        — ResolveCulprit has NO production
//	   (priority H, dependsOn T2)                caller; only ComposePrefixes is
//	                                             driven, so the positional-NNFI
//	                                             culprit-resolution engine that IS
//	                                             the design's value never runs on
//	                                             the composed main-push path
//	                                             (goal no-inert-API floor).
//	T2 prefixqueue-single-writer-race-safety   — PrefixQueue mutates lanes/window
//	   (priority H)                              with NO synchronization; the
//	                                             moment a concurrent driver is
//	                                             wired this is the silent
//	                                             lost-work class (948/949).
//	T3 prefixqueue-nnfi-postejection-reverify   — positional NNFI can land a
//	   (priority M)                              poisoned COMPOSITE: solo-green
//	                                             lanes whose union is red both
//	                                             land because the composed set is
//	                                             never re-verified as a whole.
//
// SUT SURFACE the Builder must add WITHOUT modifying this file (the RED contract).
// The one symbol that does not yet exist — ship.LandPrefixes — makes this package
// FAIL TO COMPILE now, which is the correct greenfield RED per go/acs/README.md
// ("a predicate package that fails to compile is a HARD suite error, never a
// silent PASS"). T2/T3 predicates ALSO encode behavioral RED that holds once the
// missing symbol lands (verified against the current tree: a cross-group poisoned
// composite lands today, and unsynchronized concurrent Enqueue loses appends
// 30/30 rounds).
//
//	T1 (new, package go/internal/phases/ship — the composed-path DRIVER that makes
//	    ResolveCulprit non-inert; it must ROUTE through PrefixQueue.ResolveCulprit,
//	    not reimplement NNFI inline):
//	  func LandPrefixes(cfg policy.FleetConfig, lanes []fleet.LaneCandidate,
//	                    verify func(laneIDs []string) bool) (landed, ejected []string)
//	  //   prefix-queue => enqueue the lanes, resolve culprits via
//	  //                   fleet.PrefixQueue.ResolveCulprit(verify); return
//	  //                   landed / ejected. Empty lane set => nil, nil.
//	  // AND at least one non-_test .go file under internal/phases/ship must
//	  // reference ResolveCulprit (the wiring proof for the no-inert-API floor).
//
//	T2 (go/internal/fleet/prefixqueue.go): guard lanes/window with a sync.Mutex
//	    (or sync.RWMutex) across Enqueue/OnGreen/OnRed/Window so concurrent
//	    Enqueues never lose an append and the AIMD window never tears below floor.
//
//	T3 (go/internal/fleet/prefixqueue.go): after resolving, re-verify the surviving
//	    landed set as a whole; if it fails, trim rather than land a poisoned
//	    composite — invariant: verify(landed) is ALWAYS true. The single-group
//	    independent-failure path (poisoned middle lane) stays unchanged.
//
// PREDICATE STYLE (cycle-85 anti-gaming): every load-bearing predicate CALLS the
// SUT and asserts on its return value / observed side effect. The two structural
// checks (ship caller present; mutex token present) are SUPPORTING only — each
// sits beside a behavioral assertion in the same AC and matches the scout's stated
// verifiableBy grep, so neither is a sole load-bearing source-grep.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C982_001 (innocent lanes land through the live ship driver),
//	           C982_003 (concurrent Enqueue preserves every lane),
//	           C982_005 (independent-failure NNFI still ejects the true culprit).
//	NEGATIVE → C982_001 (poisoned middle lane must NOT land; NNFI budget is LINEAR
//	           — anti-bisection), C982_002 (ResolveCulprit must have a real ship
//	           caller — an inline reimplementation leaving it inert fails),
//	           C982_005 (a poisoned CROSS-GROUP composite must NOT land — the exact
//	           F2 gap; current code lands [A,B] here).
//	EDGE     → C982_001 (empty lane set => nil/nil), C982_003 (AIMD window floors
//	           at 1 under concurrent reds), C982_005 (single-lane / empty group).
//	SEMANTIC → live-ship-wiring / single-writer-safety / poisoned-composite-trim
//	           are three DISTINCT behaviors, not one restated.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC-T1b live ship driver ejects culprit, lands innocents (linear NNFI) → C982_001 (predicate)
//	AC-T1c empty lane set => nil landed/ejected                          → C982_001 (predicate)
//	AC-T1a a ship non-test file references ResolveCulprit (wiring proof)  → C982_002 (predicate)
//	AC-T2a concurrent Enqueue never loses an append                      → C982_003 (predicate)
//	AC-T2c AIMD window never drops below floor 1 under concurrent reds    → C982_003 (predicate)
//	AC-T2b prefixqueue.go guards shared state with sync.Mutex/RWMutex     → C982_004 (predicate)
//	AC-T3a a poisoned composite never lands (verify(landed) always true)  → C982_005 (predicate)
//	AC-T3b independent-failure single-group NNFI path unchanged           → C982_005 (predicate)
//	AC-T3c design-note comment documents the NNFI positional limitation   → manual+checklist (Auditor)
package cycle982

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// contains reports whether ids includes id.
func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// C982_001 — AC-T1b/AC-T1c: the ship-phase driver ships ResolveCulprit LIVE on the
// composed main-push path. In prefix-queue mode a poisoned middle lane is ejected
// while the innocent lanes land, resolved in a LINEAR verify budget (positional
// NNFI, no bisection). This is the composed-path WIRING PROOF for the no-inert-API
// floor: the driver's behavior IS ResolveCulprit's behavior. Empty lane set edge.
func TestC982_001_LiveShipDriverResolvesCulprit(t *testing.T) {
	// prefix-queue policy so the driver routes through the composer (resolved via
	// the real policy resolver, matching the cycle-981 wiring predicate).
	cfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()

	// Three composable (non-iffy, disjoint-file) lanes → one group. L2 poisoned.
	lanes := []fleet.LaneCandidate{
		{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}},
		{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}},
		{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}},
	}
	calls := 0
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2") // a prefix fails iff it holds the poisoned lane
	}

	landed, ejected := ship.LandPrefixes(cfg, lanes, verify)

	// POSITIVE: the two innocent lanes land through the live driver.
	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land via ship.LandPrefixes, got landed=%v", landed)
	}
	// NEGATIVE (anti-no-op): the poisoned lane must NOT land, and is the sole ejection.
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	// NEGATIVE (anti-bisection): NNFI resolves in a LINEAR verify budget. A driver
	// that reimplemented landing with a bisection/brute-force sweep blows this.
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI (<=6), not a sweep", calls)
	}

	// EDGE: an empty lane set yields no landings and no ejections (no panic).
	if l, e := ship.LandPrefixes(cfg, nil, verify); len(l) != 0 || len(e) != 0 {
		t.Errorf("empty lane set => landed=%v ejected=%v, want both empty", l, e)
	}
}

// C982_002 — AC-T1a: the no-inert-API wiring proof. ResolveCulprit must be
// referenced from a NON-test .go file under internal/phases/ship — i.e. it has a
// real production caller on the composed path, not merely a test driver. This is
// the structural half of T1 (verifiableBy: grep shows ResolveCulprit under
// internal/phases/ship non-test); it sits beside the behavioral C982_001, so it
// is a supporting check, not a sole load-bearing source-grep.
func TestC982_002_ResolveCulpritHasShipCaller(t *testing.T) {
	root := acsassert.RepoRoot(t) // skips cleanly when not in a git work tree
	shipDir := root + "/go/internal/phases/ship"
	stdout, _, code, err := acsassert.SubprocessOutput("grep", "-rl", "--include=*.go", "ResolveCulprit", shipDir)
	if err != nil || code != 0 {
		t.Fatalf("ResolveCulprit has no caller under %s (grep code=%d err=%v) — the composer is still INERT, violating the no-inert-API floor", shipDir, code, err)
	}
	// At least one matching file must be a NON-test production file.
	prod := false
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line != "" && !strings.HasSuffix(line, "_test.go") {
			prod = true
			break
		}
	}
	if !prod {
		t.Errorf("ResolveCulprit is referenced only from *_test.go under ship (%q) — a test-only caller leaves the composed production path inert", strings.TrimSpace(stdout))
	}
}

// C982_003 — AC-T2a/AC-T2c: single-writer safety. Concurrent Enqueues must never
// lose an append (the 948/949 silent-lost-work class), and concurrent AIMD reds
// must never tear the window below its floor of 1. Verified RED against the
// current unsynchronized tree (30/30 rounds lost appends); after the Builder adds
// the mutex this holds deterministically. The acs suite does NOT run with -race
// (go test -json -tags acs, no -race), so we detect lost work via the length
// invariant under high contention; T2's -race unit regression is separate.
func TestC982_003_ConcurrentEnqueueNoLostWork(t *testing.T) {
	const rounds = 20
	const n = 300
	for r := 0; r < rounds; r++ {
		q := fleet.NewPrefixQueue()
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				q.Enqueue(fleet.LaneCandidate{
					ID:    fmt.Sprintf("L%d", i),
					Tier:  fleet.TierMaybe,
					Files: []string{fmt.Sprintf("go/internal/f%d/f.go", i)},
				})
			}(i)
		}
		wg.Wait()
		// ComposePrefixes emits exactly one prefix per enqueued lane, so its length
		// is the FIFO length — a torn/lost append shows up as a short count.
		if got := len(q.ComposePrefixes()); got != n {
			t.Fatalf("round %d: %d lanes survived %d concurrent Enqueues — lost appends (unsynchronized single-writer)", r, got, n)
		}
	}

	// EDGE: concurrent AIMD reds must never drive the window below its floor of 1.
	q := fleet.NewPrefixQueue()
	for i := 0; i < 5; i++ {
		q.OnGreen() // push the window well above the floor first
	}
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() { defer wg.Done(); q.OnRed() }()
	}
	wg.Wait()
	if got := q.Window(); got < 1 {
		t.Fatalf("AIMD window = %d after concurrent reds — floor of 1 breached (torn read-modify-write)", got)
	}
}

// C982_004 — AC-T2b: the structural half of T2. prefixqueue.go must guard its
// shared state with a real sync.Mutex/RWMutex (the scout's committed verifiableBy:
// grep -nE 'sync\.(Mutex|RWMutex)' internal/fleet/prefixqueue.go). Supporting the
// behavioral C982_003, not a sole source-grep.
func TestC982_004_PrefixQueueGuardsSharedState(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := root + "/go/internal/fleet/prefixqueue.go"
	if !acsassert.FileExists(t, path) {
		t.Fatalf("prefixqueue.go missing at %s", path)
	}
	if !acsassert.FileMatchesRegex(t, path, `sync\.(Mutex|RWMutex)`) {
		t.Errorf("prefixqueue.go declares no sync.Mutex/RWMutex — shared lanes/window are unguarded")
	}
}

// C982_005 — AC-T3a/AC-T3b: no poisoned composite may land. Positional NNFI can
// pass every incremental prefix yet leave a landed SET that fails as a whole —
// two solo-green lanes whose union is red (F2). The invariant: verify(landed) is
// ALWAYS true, so the composer must re-verify and trim rather than land the
// poisoned composite. Verified RED against the current tree: today ResolveCulprit
// lands [A,B] even though verify([A,B]) is false. The independent-failure
// single-group path (poisoned middle lane) must stay unchanged.
func TestC982_005_NoPoisonedCompositeLands(t *testing.T) {
	// NEGATIVE — cross-group composite poison. Two iffy lanes (each its own group,
	// each solo-green) whose UNION is red. Current code lands both; the fix must
	// trim so the landed set re-verifies clean.
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "A", Tier: fleet.TierIffy, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "B", Tier: fleet.TierIffy, Files: []string{"go/internal/b/b.go"}})
	poison := func(laneIDs []string) bool {
		return !(contains(laneIDs, "A") && contains(laneIDs, "B")) // red iff A and B land together
	}
	landed, _ := q.ResolveCulprit(poison)
	if !poison(landed) {
		t.Errorf("landed set %v fails full re-verification — a poisoned composite landed (F2: verify(landed) must always hold)", landed)
	}
	if contains(landed, "A") && contains(landed, "B") {
		t.Errorf("both A and B landed (%v) despite their union failing — poisoned composite not trimmed", landed)
	}

	// POSITIVE / regression — the independent-failure single-group path is
	// UNCHANGED: a poisoned middle lane in one composable group is ejected while
	// the innocent lanes land, and the landed set re-verifies clean.
	q2 := fleet.NewPrefixQueue()
	q2.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/x/x.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/y/y.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/z/z.go"}})
	mid := func(laneIDs []string) bool { return !contains(laneIDs, "L2") }
	landed2, ejected2 := q2.ResolveCulprit(mid)
	if !contains(landed2, "L1") || !contains(landed2, "L3") || contains(landed2, "L2") {
		t.Errorf("independent-failure path changed: landed=%v, want L1,L3 land and L2 eject", landed2)
	}
	if len(ejected2) != 1 || ejected2[0] != "L2" {
		t.Errorf("independent-failure path changed: ejected=%v, want exactly [L2]", ejected2)
	}
	if !mid(landed2) {
		t.Errorf("independent-failure landed set %v fails re-verification", landed2)
	}

	// EDGE — single lane and empty queue resolve cleanly (no panic, no spurious eject).
	q3 := fleet.NewPrefixQueue()
	q3.Enqueue(fleet.LaneCandidate{ID: "S", Tier: fleet.TierMaybe, Files: []string{"go/internal/s/s.go"}})
	if l, e := q3.ResolveCulprit(func([]string) bool { return true }); len(l) != 1 || l[0] != "S" || len(e) != 0 {
		t.Errorf("single green lane => landed=%v ejected=%v, want [S] / none", l, e)
	}
	if l, e := fleet.NewPrefixQueue().ResolveCulprit(func([]string) bool { return true }); len(l) != 0 || len(e) != 0 {
		t.Errorf("empty queue => landed=%v ejected=%v, want both empty", l, e)
	}
}
