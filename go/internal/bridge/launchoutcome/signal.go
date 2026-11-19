package launchoutcome

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const (
	CodeExitSafetyGate              signalcenter.Code = "BRIDGE_EXIT_SAFETY_GATE"
	CodeExitCostLeak                signalcenter.Code = "BRIDGE_EXIT_COST_LEAK"
	CodeExitBadFlags                signalcenter.Code = "BRIDGE_EXIT_BAD_FLAGS"
	CodeExitREPLBootTimeout         signalcenter.Code = "BRIDGE_EXIT_REPL_BOOT_TIMEOUT"
	CodeExitArtifactTimeout         signalcenter.Code = "BRIDGE_EXIT_ARTIFACT_TIMEOUT"
	CodeExitUnknownPrompt           signalcenter.Code = "BRIDGE_EXIT_UNKNOWN_PROMPT"
	CodeExitRespondLoopGuard        signalcenter.Code = "BRIDGE_EXIT_RESPOND_LOOP_GUARD"
	CodeExitRequiredTierUnavailable signalcenter.Code = "BRIDGE_EXIT_REQUIRED_TIER_UNAVAILABLE"
	CodeExitCommandTimeout          signalcenter.Code = "BRIDGE_EXIT_COMMAND_TIMEOUT"
	CodeExitMissingBinary           signalcenter.Code = "BRIDGE_EXIT_MISSING_BINARY"
	CodeExitSignalDeath             signalcenter.Code = "BRIDGE_EXIT_SIGNAL_DEATH"
	CodeExitDriverError             signalcenter.Code = "BRIDGE_EXIT_DRIVER_ERROR"
	CodeExitModelMismatch           signalcenter.Code = "BRIDGE_EXIT_MODEL_MISMATCH"
	CodeExitSeatTokenRefused        signalcenter.Code = "BRIDGE_EXIT_SEAT_TOKEN_REFUSED"
	CodeExitQuotaWall               signalcenter.Code = "BRIDGE_EXIT_QUOTA_WALL"
)

func init() {
	const common = ". The reason is the classified launch error (parsed by outcome records and retry backoff). Fields: exit_code, cause_code, transient, ctx_cancelled, launch_error (when written), call_id, cli, agent, step=classify"
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitSafetyGate, "a launch died in a safety gate (exit 2: for example, without --allow-bypass, or --human-input with no audit). Plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitCostLeak, "a launch refused to leak a forbidden credential into the inner CLI environment (exit 3). Plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitBadFlags, "a launch died in the validate gauntlet: bad flags, missing profile, unreadable or empty prompt, or no driver (exit 10). Plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitREPLBootTimeout, "a tmux REPL never showed its prompt marker within the boot budget (exit 80). Transient; the boot strike is recorded before this event"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitArtifactTimeout, "the artifact never appeared within the wait window (exit 81); wraps core.ErrArtifactTimeout. Sub-cause rides cause_code (context_cancelled, pane_lost, completion_detector_error, submit_wedged, transient_upstream, review_stop, review_pause, incomplete)"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitUnknownPrompt, "the auto-responder escalated on an interactive prompt or a checkpoint corroborated a quota wall (exit 85). Transient; cause_code names the pattern (rate_limit, model_unsupported, or unknown_prompt)"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitRespondLoopGuard, "the auto-respond loop guard tripped (exit 86). Transient"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitModelMismatch, "a tmux REPL booted a model outside model_family or showed no readable model label (exit 87). Plain failure that walks to the next CLI; BRIDGE_DISPATCH_MODEL_MISMATCH carries expected and observed"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitSeatTokenRefused, "a claude seat refused to start because the seat token is empty, unreadable, not mode 0600, invalid, or unconfined (exit 88). Plain failure; no retry or fallback fixes it"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitQuotaWall, "a launch exited 85 because the pane showed a quota or usage-limit wall (cause_code rate_limit or exhausted). Transient; named for the quota so pause reads the wall"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitRequiredTierUnavailable, "--require-full was set and the full model tier is unavailable (exit 99). Plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitCommandTimeout, "the driver was killed by a command-level timeout (exit 124, gnu timeout convention). Transient infra weather, sibling of 81"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitMissingBinary, "a required external binary is missing (exit 127). Deliberately plain; family fallback sees raw 127"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitSignalDeath, "the driver died of a signal or failed to start (exit -1): transient under cancelled context, plain with a live one. Ledger cause stays driver_error"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitDriverError, "a launch exited with a code the table does not name. Plain failure, ledger cause driver_error"+common)
}
