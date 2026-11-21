package logread

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (r Reader) RecordShadow(asPhase string, access *phasespec.LogAccess, layer Layer, verb string) bool {
	if access.Allows(string(layer)) {
		return false
	}
	if r.Signals != nil {
		r.Signals.Emit(signalcenter.Event{
			Cycle: r.Cycle, Phase: asPhase, Module: signalcenter.ModuleLog, Origin: "Reader.RecordShadow",
			Kind: signalcenter.KindRunnerWarning, Code: signalcenter.CodeLogAccessShadow, Severity: signalcenter.SeverityWarn,
			Reason: "phase " + asPhase + " read layer " + string(layer) + ", which its log_access does not declare; shadow mode lets the read go on",
			Fields: map[string]string{"layer": string(layer), "declared": strings.Join(access.Layers, ","), "verb": verb},
		})
		r.Signals.Flush()
	}
	return true
}

func (r Reader) DispatchBlock() []string {
	lines, err := r.Digest()
	if err != nil {
		lines = nil
	}
	return append(lines, "Use evolve logs (index, digest, events, failures, show) for a deeper query; each verb is bounded and never returns a whole file.")
}
