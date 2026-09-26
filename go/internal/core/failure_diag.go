package core

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// No nil-orchestrator branch: every abort site already dereferences cr.o
// first, so a nil orchestrator would already have panicked there.
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

// writePhaseFailureDiag is best-effort: a write failure is the unit's own
// signal and never masks the phase error.
func (o *Orchestrator) writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int) {
	o.failureDiag().Write(workspace, phase, cycle, phaseErr, attempts)
}

// DeliveryFailureCause reports the classified prompt-delivery failure reason,
// or "" when err is not an evidenced delivery failure.
func DeliveryFailureCause(err error) string {
	return failurediag.DeliveryFailureCause(err, isArtifactTimeout)
}
