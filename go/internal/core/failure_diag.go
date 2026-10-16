package core

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (o *Orchestrator) failureDiag() *failurediag.Writer {
	if o.diag == nil {
		o.diag = o.wiredFailureDiag()
	}
	return o.diag
}

func (o *Orchestrator) wiredFailureDiag() *failurediag.Writer {
	return failurediag.NewWriter(func() time.Time { return o.now() }, isArtifactTimeout,
		failurediag.WithSignals(func() *signalcenter.Center { return o.signals }))
}

func (o *Orchestrator) writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int) {
	o.failureDiag().Write(workspace, phase, cycle, phaseErr, attempts)
}

func DeliveryFailureCause(err error) string {
	return failurediag.DeliveryFailureCause(err, isArtifactTimeout)
}
