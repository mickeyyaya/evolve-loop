package audit

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type shadowEmitter func() *signalcenter.Center

func (s shadowEmitter) emit(e signalcenter.Event) {
	if s != nil {
		s().Emit(e)
	}
}
