package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeGuardDenied signalcenter.Code = "ISOLATION_GUARD_DENIED"

const CauseGuardRefused = llmroute.CauseGuardRefused

const (
	DenialFieldGuard   = "guard"
	DenialFieldPath    = "path"
	DenialFieldCWD     = "cwd"
	DenialFieldNeutral = "neutral"
	DenialFieldReason  = "reason"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeGuardDenied,
		"a guard hook denied a tool call (exit 2). Emitted by guards.EmitDenial into <evolve-dir>/signals.ndjson. fields.guard, fields.path (the file, else the command), fields.cwd, fields.neutral (true only for a guard every CLI family meets the same way), fields.reason. The bridge reads neutral denials after a stall to end the dispatch chain with the cause guard_refused")
}
