package mergewindow

import (
	"fmt"
	"time"
)

type WaitingPR struct {
	Number string
	Wait   time.Duration
}

type Pool struct {
	Live, Width int
}

type EarlyBoundary struct {
	Due   bool
	PR    string
	Wait  time.Duration
	Bound time.Duration
	Pool  Pool
}

func DecideEarlyBoundary(waiting []WaitingPR, pool Pool, bound time.Duration) EarlyBoundary {
	e := EarlyBoundary{Bound: bound, Pool: pool}
	for _, w := range waiting {
		if e.PR == "" || w.Wait > e.Wait {
			e.PR, e.Wait = w.Number, w.Wait
		}
	}
	e.Due = e.PR != "" && e.Wait > bound && pool.Live < pool.Width
	return e
}

func (e EarlyBoundary) Line() string {
	pool := fmt.Sprintf("pool %d/%d lanes", e.Pool.Live, e.Pool.Width)
	if e.PR == "" {
		return "no early boundary: no green boundary PR waits; " + pool
	}
	wait := fmt.Sprintf("#%s waited %s (bound %s); %s", e.PR, e.Wait, e.Bound, pool)
	switch {
	case e.Due:
		return fmt.Sprintf("early boundary: %s: run evolve loop-stop --wait, then evolve wave next --merge %s", wait, e.PR)
	case e.Pool.Live >= e.Pool.Width:
		return "no early boundary: " + wait + " is full"
	}
	return "no early boundary: " + wait
}
