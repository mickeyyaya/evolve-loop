// Package bridge is the native-Go port of tools/agent-bridge, the multi-CLI
// agent dispatch layer: it realizes a phase's launch intent into per-CLI
// flags, drives the REPL and reports the outcome through a single Engine.
package bridge

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
