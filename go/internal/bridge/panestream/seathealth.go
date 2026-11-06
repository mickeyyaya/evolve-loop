package panestream

import "time"

type SeatHealth int

const (
	SeatWorking SeatHealth = iota + 1
	SeatBlocked
	SeatStalled
	SeatZombie
	SeatIdle
)

var seatHealthNames = map[SeatHealth]string{
	SeatWorking: "working",
	SeatBlocked: "blocked",
	SeatStalled: "stalled",
	SeatZombie:  "zombie",
	SeatIdle:    "idle",
}

func (h SeatHealth) String() string {
	if name, ok := seatHealthNames[h]; ok {
		return name
	}
	return "unknown"
}

type SeatFacts struct {
	SessionAlive      bool
	RunOpen           bool
	Busy              bool
	DeliverableExists bool
	SinceProgress     time.Duration
	StallWindow       time.Duration
}

func ClassifySeat(f SeatFacts) SeatHealth {
	switch {
	case f.DeliverableExists:
		return SeatIdle
	case !f.SessionAlive && f.RunOpen:
		return SeatZombie
	case !f.SessionAlive:
		return SeatIdle
	case !f.Busy:
		return SeatBlocked
	case f.SinceProgress > f.StallWindow:
		return SeatStalled
	}
	return SeatWorking
}
