package inboxmover

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"

type RepairStamp = lifecycle.RepairStamp

func FileRepair(opts Options, raw []byte, stamp RepairStamp) (FileResult, error) {
	return withMover(opts, func(m *lifecycle.Mover) (FileResult, error) { return m.FileRepair(raw, stamp) })
}
