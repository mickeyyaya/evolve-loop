package fleet

import (
	"fmt"
	"sync"
)

type RiskTier int

const (
	TierRollup RiskTier = iota
	TierMaybe
	TierIffy
)

type LaneCandidate struct {
	ID    string
	Tier  RiskTier
	Files []string
}

type PrefixQueue struct {
	mu     sync.Mutex
	lanes  []LaneCandidate
	window int
}

func NewPrefixQueue() *PrefixQueue {
	return &PrefixQueue{window: 3}
}

func (q *PrefixQueue) Enqueue(c LaneCandidate) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.lanes = append(q.lanes, c)
}

func (q *PrefixQueue) Window() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.window
}

func (q *PrefixQueue) OnGreen() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.window++
}

func (q *PrefixQueue) OnRed() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if w := q.window / 2; w > 1 {
		q.window = w
	} else {
		q.window = 1
	}
}

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
	for len(landed) > 0 && !verify(landed) {
		last := len(landed) - 1
		ejected = append(ejected, landed[last])
		landed = landed[:last]
	}
	return landed, ejected
}

type LandingMode string

const (
	LandingPerLane     LandingMode = "per-lane"
	LandingPrefixQueue LandingMode = "prefix-queue"
)

func DefaultLandingMode() LandingMode {
	return LandingPerLane
}

func ParseLandingMode(s string) (LandingMode, error) {
	switch LandingMode(s) {
	case LandingPerLane, LandingPrefixQueue:
		return LandingMode(s), nil
	default:
		return "", fmt.Errorf("invalid landing mode %q: want %q or %q", s, LandingPerLane, LandingPrefixQueue)
	}
}
