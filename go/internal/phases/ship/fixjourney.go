package ship

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func emitFixProof(opts *Options) {
	if opts.DryRun {
		return
	}
	assessment := journeys.Assess(opts.ProjectRoot, opts.CommitMessage)
	if assessment.Problem == "" {
		return
	}
	subject, _, _ := strings.Cut(opts.CommitMessage, "\n")
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleFixProof, Origin: "emitFixProof", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: signalcenter.CodeFixProofMissing,
		Reason: "a fix commit has no usable Fix-Journey trailer (" + string(assessment.Problem) + ")",
		Fields: map[string]string{"problem": string(assessment.Problem), "subject": subject},
	})
}
