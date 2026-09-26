package fleet

import (
	"fmt"
	"sync"
)

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

// LaneCandidate is a PASS lane awaiting landing; Files are repo-relative and detect overlap-zone conflicts.
type LaneCandidate struct {
	ID    string
	Tier  RiskTier
	Files []string
}

// PrefixQueue is the single-writer landing composer: a FIFO of PASS lanes plus the AIMD window.
// See ADR-0078.
type PrefixQueue struct {
	// mu guards lanes and window: drivers enqueue and report outcomes from several goroutines.
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

// groups splits the FIFO into composable groups; a TierIffy or overlapping lane starts a new one.
// The caller must hold q.mu.
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

// ComposePrefixes returns each group's cumulative lane-ID prefixes, in queue order.
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

// ResolveCulprit lands lanes by positional NNFI, ejecting each lane that turns a known-good set red.
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
	// Green groups can still form a red union, so verify(landed) is restored by trimming
	// from the tail; the ejected lane is positional, not necessarily the culprit.
	for len(landed) > 0 && !verify(landed) {
		last := len(landed) - 1
		ejected = append(ejected, landed[last])
		landed = landed[:last]
	}
	return landed, ejected
}

// LandingMode is the fleet.landing policy vocabulary for how PASS lanes reach main.
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

// ParseLandingMode validates s against the landing-mode vocabulary, rejecting unknown and empty values.
func ParseLandingMode(s string) (LandingMode, error) {
	switch LandingMode(s) {
	case LandingPerLane, LandingPrefixQueue:
		return LandingMode(s), nil
	default:
		return "", fmt.Errorf("invalid landing mode %q: want %q or %q", s, LandingPerLane, LandingPrefixQueue)
	}
}
