//go:build acs

// Package cycle975 materializes the cycle-975 acceptance criteria for the sole
// inbox item this fleet lane is pinned to: prefix-speculation-landing-queue
// (.evolve/inbox/2026-07-13T14-21-00Z-prefix-speculation-landing-queue.json,
// weight 0.93, campaign merge-efficiency-2026-07). Per R9.3 no predicate here
// binds to any other lane's items — fleet_scope pins this lane to exactly this id.
//
// DESIGN (item summary, Zuul/GitHub-queue prefix model). Lanes never push main.
// A single-writer composer (Cognition single-writer principle) maintains a FIFO
// of PASS lane candidates and builds composed candidate trees as queue PREFIXES
// (L1, L1+L2, L1+L2+L3), verifying them concurrently against the native gate set.
// First failing prefix names the culprit positionally (Zuul NNFI); lanes behind
// re-form without it — no bisection subsystem. The window is an AIMD control loop
// (start 3, +1 per green landing, halve on red, floor 1). Lanes are risk-tiered
// like Rust rollups: iffy (core/cross-cutting) and overlap-zone lanes get a solo
// prefix slot. Landing strategy is policy config (fleet.landing: per-lane |
// prefix-queue), NOT an env flag (standing rule no_feature_flags_use_design_patterns).
//
// SUT SURFACE the Builder must add to package go/internal/fleet WITHOUT modifying
// this file (this is the RED contract — the package does not exist yet, so this
// predicate package FAILS TO COMPILE now, which is the correct greenfield RED per
// go/acs/README.md "a predicate package that fails to compile is a HARD suite
// error"):
//
//	type RiskTier int
//	const ( TierRollup RiskTier = iota; TierMaybe; TierIffy )
//	type LaneCandidate struct { ID string; Tier RiskTier; Files []string }
//	type PrefixQueue struct { ... }
//	func NewPrefixQueue() *PrefixQueue
//	func (q *PrefixQueue) Enqueue(c LaneCandidate)
//	func (q *PrefixQueue) Window() int            // AIMD window, starts at 3
//	func (q *PrefixQueue) OnGreen()               // window += 1
//	func (q *PrefixQueue) OnRed()                 // window = max(1, window/2)
//	func (q *PrefixQueue) ComposePrefixes() [][]string  // prefix k = lane IDs [0..k]
//	func (q *PrefixQueue) ResolveCulprit(verify func(laneIDs []string) bool) (landed, ejected []string)
//	type LandingMode string
//	const ( LandingPerLane LandingMode = "per-lane"; LandingPrefixQueue LandingMode = "prefix-queue" )
//	func DefaultLandingMode() LandingMode         // compiled default = per-lane
//	func ParseLandingMode(s string) (LandingMode, error)
//
// PREDICATE STYLE (cycle-85 anti-gaming rule): every predicate CALLS the SUT and
// asserts on its return value — no source-grep predicate exists here. go/internal
// is importable from go/acs (cycle-962 precedent imports internal/core). The
// composer is pure/deterministic logic, so the ACS package IS the behavioral test
// and the Builder cannot game it by weakening a colocated unit test — the
// assertions live here, out of the Builder's edit surface.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C975_001 (good lanes land around a poisoned middle), C975_002
//	           (window grows on green), C975_004 (canonical modes parse).
//	NEGATIVE → C975_001 (the poisoned lane must NOT land — strongest anti-no-op:
//	           a composer that lands everything fails this), C975_003 (an iffy /
//	           overlap-zone lane must NEVER appear in a multi-lane prefix),
//	           C975_004 (a bogus landing mode must ERROR, never default-accept).
//	EDGE     → C975_001 (verify-call count is bounded LINEARLY — proves NNFI, not
//	           an exponential powerset/bisection sweep), C975_002 (window floors at
//	           1 and never below under repeated reds).
//	SEMANTIC → culprit-ejection / AIMD-window / risk-tier-slotting / config-
//	           vocabulary are four DISTINCT behaviors, each asserted apart.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 TestPrefixQueue_PositionalCulpritEjectsAndReforms  → C975_001 (predicate)
//	AC2 TestPrefixQueue_AIMDWindowAdaptsToPassRate         → C975_002 (predicate)
//	AC3 TestPrefixQueue_IffyTierGetsSoloSlot + overlap-zone→ C975_003 (predicate)
//	AC4 single-writer only main-push path + ledger chaining→ manual+checklist (Auditor)
//	AC5 batch soak width 3: zero AUDIT_BINDING_HEAD_MOVED,  → manual+checklist (Auditor)
//	    watches==prefixes, go test -race PASS, apicover clean
//	AC6 config via policy fleet.landing (per-lane|prefix-queue), not env flag
//	                                                        → C975_004 (predicate)
package cycle975

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
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

// C975_001 — AC1: positional culprit ejection + reform, no bisection.
//
// Three PASS lanes are enqueued; the MIDDLE one (L2) is poisoned. The composer
// must land the two good lanes (L1, L3) and eject exactly the culprit (L2),
// naming it positionally from the first failing prefix — WITHOUT a bisection
// sweep. The verify-call budget is asserted linear in the lane count: an
// exponential powerset/bisection resolver would blow past it.
func TestC975_001_PositionalCulpritEjectsAndReforms(t *testing.T) {
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	calls := 0
	// A composed prefix fails iff it contains the poisoned lane L2.
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2")
	}

	landed, ejected := q.ResolveCulprit(verify)

	// POSITIVE: both good lanes land, in FIFO order.
	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land, got landed=%v", landed)
	}
	// NEGATIVE (anti-no-op): the poisoned lane must NOT land.
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	// The culprit is named positionally and ejected — exactly L2.
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	// EDGE (anti-bisection): NNFI resolves in O(lanes) verify calls. A powerset /
	// bisection resolver over 3 lanes would exceed 2*n. Bound proves no bisection
	// subsystem is used (the whole point of the prefix model).
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI resolution (<=6), not a bisection sweep", calls)
	}
}

// C975_002 — AC2: AIMD window self-tunes to the pass rate.
//
// Window starts at 3; +1 per green landing; halves (integer) on red; floors at 1
// and never drops below it under repeated reds; recovers on the next green.
func TestC975_002_AIMDWindowAdaptsToPassRate(t *testing.T) {
	q := fleet.NewPrefixQueue()

	if got := q.Window(); got != 3 {
		t.Errorf("initial window = %d, want 3", got)
	}
	q.OnGreen()
	q.OnGreen()
	if got := q.Window(); got != 5 {
		t.Errorf("window after 2 greens = %d, want 5", got)
	}
	q.OnRed() // 5 -> 2
	if got := q.Window(); got != 2 {
		t.Errorf("window after red = %d, want 2 (halved from 5)", got)
	}
	q.OnRed() // 2 -> 1
	if got := q.Window(); got != 1 {
		t.Errorf("window after red = %d, want 1 (halved from 2)", got)
	}
	// EDGE: floor holds — repeated reds never drive the window below 1.
	q.OnRed()
	if got := q.Window(); got != 1 {
		t.Errorf("window after red at floor = %d, want 1 (floor)", got)
	}
	// Recovers on green.
	q.OnGreen()
	if got := q.Window(); got != 2 {
		t.Errorf("window after green from floor = %d, want 2", got)
	}
}

// C975_003 — AC3: iffy-tier and overlap-zone lanes get a solo prefix slot.
//
// Two independent checks:
//   - An iffy (core/cross-cutting) lane must never be composed with any other
//     lane: every prefix that contains it has length 1.
//   - Two lanes touching the SAME file (overlap zone) must never share a prefix.
func TestC975_003_IffyTierGetsSoloSlot(t *testing.T) {
	// Case A: iffy lane between two normal lanes.
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "IFFY", Tier: fleet.TierIffy, Files: []string{"go/internal/core/core.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	for _, prefix := range q.ComposePrefixes() {
		if contains(prefix, "IFFY") && len(prefix) != 1 {
			t.Errorf("iffy lane must be solo, found it in multi-lane prefix %v", prefix)
		}
	}

	// Case B: overlap-zone — two lanes touching the same file must not co-occur.
	q2 := fleet.NewPrefixQueue()
	q2.Enqueue(fleet.LaneCandidate{ID: "X1", Tier: fleet.TierMaybe, Files: []string{"go/internal/shared/s.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "X2", Tier: fleet.TierMaybe, Files: []string{"go/internal/shared/s.go"}})

	for _, prefix := range q2.ComposePrefixes() {
		if contains(prefix, "X1") && contains(prefix, "X2") {
			t.Errorf("overlap-zone lanes X1,X2 (shared file) must not share a prefix, got %v", prefix)
		}
	}
}

// C975_004 — AC6: landing strategy is policy config vocabulary, not an env flag.
//
// The compiled default is per-lane; both canonical modes parse; a bogus value
// ERRORS rather than silently defaulting (the negative/anti-typo dimension).
func TestC975_004_LandingModePolicyVocabulary(t *testing.T) {
	if got := fleet.DefaultLandingMode(); got != fleet.LandingPerLane {
		t.Errorf("default landing mode = %q, want %q", got, fleet.LandingPerLane)
	}
	if m, err := fleet.ParseLandingMode("per-lane"); err != nil || m != fleet.LandingPerLane {
		t.Errorf(`ParseLandingMode("per-lane") = (%q,%v), want (%q,nil)`, m, err, fleet.LandingPerLane)
	}
	if m, err := fleet.ParseLandingMode("prefix-queue"); err != nil || m != fleet.LandingPrefixQueue {
		t.Errorf(`ParseLandingMode("prefix-queue") = (%q,%v), want (%q,nil)`, m, err, fleet.LandingPrefixQueue)
	}
	// NEGATIVE: an unknown mode must be rejected, not defaulted.
	if _, err := fleet.ParseLandingMode("bogus"); err == nil {
		t.Errorf(`ParseLandingMode("bogus") = nil error, want a validation error`)
	}
	// Guard against a nil-return stub that would let strings.TrimSpace-style
	// silent coercion pass: an empty string is not a valid mode.
	if _, err := fleet.ParseLandingMode(strings.TrimSpace("  ")); err == nil {
		t.Errorf(`ParseLandingMode("") = nil error, want a validation error`)
	}
}
