// Package launchoutcome is unit 10 of the component breakdown (ADR-0103):
// what one bridge launch exit MEANS.
package launchoutcome

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	ExitOK               = 0
	ExitSafetyGate       = 2
	ExitCostLeak         = 3
	ExitBadFlags         = 10
	ExitREPLBootTimeout  = 80
	ExitArtifactTimeout  = 81
	ExitUnknownPrompt    = 85
	ExitRespondLoopGuard = 86
	ExitRequireFullUnmet = 99
	ExitCmdTimeout       = 124
	ExitMissingBinary    = 127
	ExitModelMismatch    = 87
)

const ExitSignalDeath = -1

type exitClass struct {
	code             int
	name             string
	signal           signalcenter.Code
	causeCode        string
	sentinel         error
	sentinelOnCancel bool
}

var exitClasses = []exitClass{
	{code: ExitSafetyGate, name: "safety_gate", signal: CodeExitSafetyGate, causeCode: "safety_gate"},
	{code: ExitCostLeak, name: "cost_leak", signal: CodeExitCostLeak, causeCode: "cost_leak"},
	{code: ExitBadFlags, name: "bad_flags", signal: CodeExitBadFlags, causeCode: "bad_flags"},
	{code: ExitREPLBootTimeout, name: "repl_boot_timeout", signal: CodeExitREPLBootTimeout, causeCode: "repl_boot_timeout", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitArtifactTimeout, name: "artifact_timeout", signal: CodeExitArtifactTimeout, causeCode: "artifact_timeout", sentinel: core.ErrArtifactTimeout},
	{code: ExitUnknownPrompt, name: "unknown_prompt", signal: CodeExitUnknownPrompt, causeCode: "unknown_prompt", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitRespondLoopGuard, name: "respond_loop_guard", signal: CodeExitRespondLoopGuard, causeCode: "respond_loop_guard", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitModelMismatch, name: "model_mismatch", signal: CodeExitModelMismatch, causeCode: "model_mismatch"},
	{code: ExitRequireFullUnmet, name: "required_tier_unavailable", signal: CodeExitRequiredTierUnavailable, causeCode: "required_tier_unavailable"},
	{code: ExitCmdTimeout, name: "command_timeout", signal: CodeExitCommandTimeout, causeCode: "command_timeout", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitMissingBinary, name: "missing_binary", signal: CodeExitMissingBinary, causeCode: "missing_binary"},
	{code: ExitSignalDeath, name: "signal_death", signal: CodeExitSignalDeath, causeCode: "driver_error", sentinel: core.ErrTransientBridgeFailure, sentinelOnCancel: true},
}

var driverErrorClass = exitClass{name: "driver_error", signal: CodeExitDriverError, causeCode: "driver_error"}

func classOf(code int) exitClass {
	for _, row := range exitClasses {
		if row.code == code {
			return row
		}
	}
	return driverErrorClass
}
