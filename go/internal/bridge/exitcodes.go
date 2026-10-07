// Package bridge is the native-Go port of tools/agent-bridge, the multi-CLI
// agent dispatch layer: it realizes a phase's launch intent into per-CLI
// flags, drives the REPL and reports the outcome through a single Engine.
// See docs/architecture/packages/internal-bridge.md.
package bridge

// Bridge exit codes: the numeric contract the drivers, cmd/evolve and the
// ACS predicates read.
const (
	ExitOK               = 0   // success
	ExitSafetyGate       = 2   // safety-gate (e.g. --human-input without host opt-in)
	ExitCostLeak         = 3   // cost-leak (forbidden env-var leak: ANTHROPIC_API_KEY, …)
	ExitBadFlags         = 10  // bad flags or missing required arg
	ExitREPLBootTimeout  = 80  // REPL boot timeout (*-tmux drivers)
	ExitArtifactTimeout  = 81  // artifact never appeared within the wait window
	ExitUnknownPrompt    = 85  // unknown interactive prompt (escalation report written)
	ExitRespondLoopGuard = 86  // auto-respond loop guard tripped
	ExitRequireFullUnmet = 99  // --require-full set and full tier unavailable
	ExitCmdTimeout       = 124 // driver killed by a command-level timeout (gnu `timeout` convention)
	ExitMissingBinary    = 127 // required external binary missing
	ExitModelMismatch    = 87
)
