// Package bridge is the native-Go port of tools/agent-bridge — the
// multi-CLI agent dispatch layer. It reimplements the bash bin/bridge
// launch/probe/report/doctor logic in-process behind a single Engine
// that satisfies core.Bridge, plus a CLI shim (evolve bridge ...) that
// preserves the historical `bridge <subcommand>` surface.
//
// Architecture (see docs/architecture/adr/ — bridge-go-port):
//
//   - Engine (bridge.go): Template Method — Launch() runs the fixed flow
//     validate → resolveConfig → preflight → dispatch(driver) → report.
//   - Driver (driver.go): Strategy + self-registering Registry, one per
//     --cli target. Mirrors internal/phases/registry.
//   - Seams (Deps): CmdRunner, clock, challenge-token, tmux, fs — all
//     injectable so the whole package is unit-testable with no LLM cost.
//
// The bash implementation is retained behind EVOLVE_BRIDGE_GO until
// parity is proven, then deleted.
package bridge

// Bridge exit codes — the single source of truth for the numeric
// contract. These mirror the EC_* constants in
// tools/agent-bridge/bin/bridge exactly; docs, skills, and the
// dispatcher's failure classifier depend on these values, so they are
// load-bearing and must not drift.
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
	ExitMissingBinary    = 127 // required external binary missing
)
