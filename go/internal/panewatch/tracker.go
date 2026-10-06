package panewatch

import "time"

type Frame struct {
	Hash       string
	Busy       bool
	TokenLine  string
	ModelLabel string
}

type Tracker struct {
	last   Snapshot
	primed bool
}

func NewTracker(identity Snapshot) *Tracker {
	return &Tracker{last: identity}
}

func (t *Tracker) Observe(f Frame, now time.Time) (Snapshot, bool) {
	next := t.last
	next.Busy, next.TokenLine, next.ModelLabel = f.Busy, f.TokenLine, f.ModelLabel
	progressed := !t.primed || f.Hash != t.last.ProgressHash
	if progressed {
		next.ProgressHash, next.ProgressAt = f.Hash, now
	}
	if t.primed && !progressed && next == t.last {
		return t.last, false
	}
	next.UpdatedAt = now
	t.last, t.primed = next, true
	return next, true
}

func (t *Tracker) Agent() string { return t.last.Agent }
