package core

// failure_diag.go — unit 02 (ADR-0103, design decomposition/02-failure-diagnostics.md):
// the orchestrator's seam onto the failurediag unit. Every abort site keeps
// calling writePhaseFailureDiag (a method now, so the writer reaches the
// orchestrator's clock and Signal Center); retro and the bridge binding tests
// keep calling the exported DeliveryFailureCause.

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// failureDiag returns the unit-02 writer: eager from NewOrchestrator, lazily
// built and cached for an Orchestrator assembled as a literal (the shape this
// package's tests keep). No nil-orchestrator branch: every abort site
// dereferences cr.o on the line before (recordPhaseOutcome), so a nil
// orchestrator already panics there and no such path exists.
func (o *Orchestrator) failureDiag() *failurediag.Writer {
	if o.diag == nil {
		o.diag = o.wiredFailureDiag()
	}
	return o.diag
}

// wiredFailureDiag is the ONE construction of the writer
// (TestFailureDiagWriter_OneConstructionSite): the clock and the Center are
// read live, and the gate is isArtifactTimeout ONLY — never the infra-teardown
// union (TestTimeoutOnlySites_NotWidenedToUnion).
func (o *Orchestrator) wiredFailureDiag() *failurediag.Writer {
	return failurediag.NewWriter(func() time.Time { return o.now() }, isArtifactTimeout,
		failurediag.WithSignals(func() *signalcenter.Center { return o.signals }))
}

// writePhaseFailureDiag is the seam every abort site keeps: the structured
// <phase>-failure-diag.json for a mandatory phase that aborted. Best-effort —
// a write failure is the unit's own signal and never masks the phase error.
func (o *Orchestrator) writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int) {
	o.failureDiag().Write(workspace, phase, cycle, phaseErr, attempts)
}

// DeliveryFailureCause keeps its exported signature for the retro relaunch
// (internal/phases/retro) and the bridge's format-binding tests: the
// classified prompt-delivery failure reason, or "" when err is not an
// evidenced delivery failure. The gate is isArtifactTimeout ONLY.
func DeliveryFailureCause(err error) string {
	return failurediag.DeliveryFailureCause(err, isArtifactTimeout)
}
