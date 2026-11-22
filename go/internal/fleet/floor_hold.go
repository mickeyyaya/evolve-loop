package fleet

import "sync/atomic"

type Hold interface {
	Held() bool
	Wake() <-chan struct{}
}

type FloorHold struct {
	held atomic.Bool
	wake chan struct{}
}

func NewFloorHold() *FloorHold { return &FloorHold{wake: make(chan struct{}, 1)} }

func (h *FloorHold) Held() bool { return h.held.Load() }

func (h *FloorHold) Wake() <-chan struct{} { return h.wake }

func (h *FloorHold) Set(held bool) {
	h.held.Store(held)
	h.Poke()
}

func (h *FloorHold) Poke() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}
