package fleet

import (
	"fmt"
	"sync"
)

// prefixqueue.go implements the cycle-975 inbox item prefix-speculation-landing-queue
// (campaign merge-efficiency-2026-07): a single-writer landing composer modeled on
// Zuul / GitHub merge-queue "prefix" speculation. Fleet lanes never push main
// themselves; instead a PrefixQueue holds a FIFO of PASS lane candidates and builds
// composed candidate trees as queue PREFIXES (L1, L1+L2, L1+L2+L3), which are verified
// against the native gate set. The first failing prefix names the culprit positionally
// (Zuul NNFI — No New Failures Introduced) and the lanes behind it re-form without it,
// so no bisection subsystem is needed. The window is an AIMD control loop (start 3,
// +1 per green, halve on red, floor 1). Lanes are risk-tiered like Rust rollups: an
// iffy (core / cross-cutting) lane and any overlap-zone lane (sharing a touched file
// with a lane already in the composing group) get a solo prefix slot.
//
// Landing strategy is policy config (fleet.landing: per-lane | prefix-queue), NOT an
// env flag (standing rule no_feature_flags_use_design_patterns) — see LandingMode.

// RiskTier ranks how safely a lane may be composed with others in one prefix.
type RiskTier int

const (
	// TierRollup lanes are the safest — freely composable rollup material.
	TierRollup RiskTier = iota
	// TierMaybe lanes are normal composable lanes.
	TierMaybe
	// TierIffy lanes touch core / cross-cutting surface and get a solo slot.
	TierIffy
)

// LaneCandidate is a PASS lane awaiting landing: its stable ID, its risk tier, and the
// repo-relative files it touches (used to detect overlap-zone conflicts).
type LaneCandidate struct {
	ID    string
	Tier  RiskTier
	Files []string
}

// PrefixQueue is the single-writer landing composer: a FIFO of PASS lane candidates
// plus the AIMD window controlling how many lanes it will speculate over at once.
//
// The composer is the single writer to main, but its own state (lanes/window) is
// still reached from >1 goroutine the moment a concurrent driver enqueues PASS
// lanes or reports AIMD outcomes; mu enforces the single-writer contract on that
// shared state so a torn append never silently loses a lane (the 948/949 lost-work
// class). Every exported method that reads or mutates lanes/window takes mu; the
// unexported groups() helper does NOT lock and is only ever called from a method
// that already holds it (no re-entrant re-lock, no deadlock).
type PrefixQueue struct {
	mu     sync.Mutex
	lanes  []LaneCandidate
	window int
}

// NewPrefixQueue returns an empty queue with the AIMD window at its start value of 3.
func NewPrefixQueue() *PrefixQueue {
	return &PrefixQueue{window: 3}
}

// Enqueue appends a PASS lane candidate to the FIFO.
func (q *PrefixQueue) Enqueue(c LaneCandidate) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.lanes = append(q.lanes, c)
}

// Window returns the current AIMD speculation window.
func (q *PrefixQueue) Window() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.window
}

// OnGreen records a green landing: additive increase (+1).
func (q *PrefixQueue) OnGreen() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.window++
}

// OnRed records a red landing: multiplicative decrease (halve), floored at 1.
func (q *PrefixQueue) OnRed() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if w := q.window / 2; w > 1 {
		q.window = w
	} else {
		q.window = 1
	}
}

// groups partitions the FIFO into composable groups, in order. A TierIffy lane is
// always its own group; a lane that shares a touched file with any lane already in the
// current group starts a fresh group (overlap-zone isolation). All other lanes accrete
// into the current group.
func (q *PrefixQueue) groups() [][]LaneCandidate {
	var result [][]LaneCandidate
	var cur []LaneCandidate
	curFiles := map[string]bool{}

	flush := func() {
		if len(cur) > 0 {
			result = append(result, cur)
			cur = nil
			curFiles = map[string]bool{}
		}
	}

	for _, lane := range q.lanes {
		if lane.Tier == TierIffy {
			flush()
			result = append(result, []LaneCandidate{lane})
			continue
		}
		overlap := false
		for _, f := range lane.Files {
			if curFiles[f] {
				overlap = true
				break
			}
		}
		if overlap {
			flush()
		}
		cur = append(cur, lane)
		for _, f := range lane.Files {
			curFiles[f] = true
		}
	}
	flush()
	return result
}

// ComposePrefixes returns the candidate prefix trees to verify, as lane-ID slices.
// Within each composable group the prefixes are cumulative (prefix k = the group's
// lane IDs [0..k]); iffy and overlap-zone lanes land in their own single-element group,
// so they never appear in a multi-lane prefix.
func (q *PrefixQueue) ComposePrefixes() [][]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var prefixes [][]string
	for _, g := range q.groups() {
		var ids []string
		for _, lane := range g {
			ids = append(ids, lane.ID)
			prefix := append([]string(nil), ids...)
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// ResolveCulprit lands as many lanes as possible using positional NNFI resolution.
// verify reports whether a composed set of lane IDs passes the gate set. Within each
// group the composer optimistically extends the known-good committed set one lane at a
// time: a lane that keeps the set green lands; a lane that turns it red is the culprit
// (positionally named — it is the only new addition to a known-good set) and is ejected,
// while the lanes behind it re-form and continue. This runs in O(lanes) verify calls —
// exactly one per lane — with no bisection sweep.
func (q *PrefixQueue) ResolveCulprit(verify func(laneIDs []string) bool) (landed, ejected []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, g := range q.groups() {
		var committed []string
		for _, lane := range g {
			trial := append(append([]string(nil), committed...), lane.ID)
			if verify(trial) {
				committed = trial
				landed = append(landed, lane.ID)
			} else {
				ejected = append(ejected, lane.ID)
			}
		}
	}
	// Post-ejection whole-set re-verify (T3, F2 gap). Per-group NNFI proves each
	// group's prefix green in isolation, but the UNION of independently-green
	// groups can still be red — two solo-green iffy lanes whose composite fails
	// (each is its own group, so the cross-group interaction is never speculated
	// on). Invariant to restore: verify(landed) must ALWAYS hold. Re-verify the
	// surviving set as a whole and, while it is red, eject the positionally-newest
	// landed lane and re-check.
	//
	// DESIGN NOTE — positional-NNFI limitation (AC-T3c): NNFI blames the newest
	// addition to a known-good set, so this tail-trim ejects the LAST-landed lane,
	// not necessarily the true composite-poisoning one. An innocent later lane may
	// be ejected in place of an earlier culprit. The guarantee here is only that no
	// poisoned composite LANDS (verify(landed) holds); the ejected identity is
	// positional, not causal. Precise blame would need cross-group bisection, which
	// this NNFI design deliberately trades away for a linear verify budget.
	for len(landed) > 0 && !verify(landed) {
		last := len(landed) - 1
		ejected = append(ejected, landed[last])
		landed = landed[:last]
	}
	return landed, ejected
}

// LandingMode is the policy vocabulary for how a fleet lands PASS lanes. It is set via
// policy config (fleet.landing), never an env flag.
type LandingMode string

const (
	// LandingPerLane lands each lane independently (the compiled default).
	LandingPerLane LandingMode = "per-lane"
	// LandingPrefixQueue lands lanes through the single-writer prefix composer.
	LandingPrefixQueue LandingMode = "prefix-queue"
)

// DefaultLandingMode is the compiled default landing mode: per-lane.
func DefaultLandingMode() LandingMode {
	return LandingPerLane
}

// ParseLandingMode validates s against the known landing-mode vocabulary. An unknown or
// empty value is rejected with an error rather than silently coerced to a default.
func ParseLandingMode(s string) (LandingMode, error) {
	switch LandingMode(s) {
	case LandingPerLane, LandingPrefixQueue:
		return LandingMode(s), nil
	default:
		return "", fmt.Errorf("invalid landing mode %q: want %q or %q", s, LandingPerLane, LandingPrefixQueue)
	}
}
