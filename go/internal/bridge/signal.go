package bridge

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeTokenResolverMissing  signalcenter.Code = "BRIDGE_TOKEN_RESOLVER_MISSING"
	CodeTokenResolverFailed   signalcenter.Code = "BRIDGE_TOKEN_RESOLVER_FAILED"
	CodeTokenUsageWarning     signalcenter.Code = "BRIDGE_TOKEN_USAGE_WARNING"
	CodeContextFillHigh       signalcenter.Code = "BRIDGE_CONTEXT_FILL_HIGH"
	CodeTelemetryAppendFailed signalcenter.Code = "BRIDGE_TELEMETRY_APPEND_FAILED"
	CodeTelemetryTripwire     signalcenter.Code = "BRIDGE_TELEMETRY_TRIPWIRE"
	CodeTurnSumMismatch       signalcenter.Code = "BRIDGE_TURN_SUM_MISMATCH"
	CodeTurnAppendFailed      signalcenter.Code = "BRIDGE_TURN_APPEND_FAILED"
	CodeTokensRowUnattributed signalcenter.Code = "TOKENS_ROW_UNATTRIBUTED"

	CodeBootStrikeClearFailed    signalcenter.Code = "BRIDGE_BOOT_STRIKE_CLEAR_FAILED"
	CodeBootStrikeRecordFailed   signalcenter.Code = "BRIDGE_BOOT_STRIKE_RECORD_FAILED"
	CodeLaunchErrorPersistFailed signalcenter.Code = "BRIDGE_LAUNCH_ERROR_PERSIST_FAILED"
	CodeResultReadFailed         signalcenter.Code = "BRIDGE_RESULT_READ_FAILED"

	CodeFreshSessionRetry signalcenter.Code = "BRIDGE_FRESH_SESSION_RETRY"

	CodeCompletedOnWorktreeEvidence signalcenter.Code = "BRIDGE_COMPLETED_ON_WORKTREE_EVIDENCE"

	CodePaneStagnant  signalcenter.Code = "LIVENESS_PANE_STAGNANT"
	CodePaneHung      signalcenter.Code = "LIVENESS_PANE_HUNG"
	CodePaneExhausted signalcenter.Code = "LIVENESS_PANE_EXHAUSTED"
	CodePhaseStalled  signalcenter.Code = "LIVENESS_PHASE_STALLED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenResolverMissing, "the engine was built without a token resolver; lifecycle and outcome records continue without token counts (fail-open)")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenResolverFailed, "the token resolver returned an error for a completed try; usage recorded as resolver-error")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenUsageWarning, "the token resolver measured the try with a caveat (invalid counters, partial measurement); the caveat is the reason")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeContextFillHigh, "a try's context fill crossed the configured warn threshold; the reason names the fill and the threshold")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTelemetryAppendFailed, "the per-try telemetry record failed to append to the workspace ledger; fields name the path")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTelemetryTripwire, "a successful try ran past the tripwire threshold with no measurable token usage — telemetry blind spot, not a phase failure")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTurnSumMismatch, "the per-turn token counts of a call do not sum to the llm-calls row. The row is unchanged and the turn sidecar stays for diagnosis. Fields: call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTurnAppendFailed, "the per-turn lines of a call failed to append to llm-turns.ndjson. The launch and llm-calls row stay unchanged (telemetry is fail-open). Fields: call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleTokens, CodeTokensRowUnattributed, "a cycle workspace wrote an llm-calls row whose request carried no cycle number, so it cannot be attributed. Emitted at the enforce stage only. Fields: call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeBootStrikeClearFailed, "the boot-strike store failed to clear the driver's consecutive boot-timeout strike after a non-80 exit (REPL booted). The strike count can stay stale and bench early. fields step=clear_boot_strike, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeBootStrikeRecordFailed, "the boot-strike store failed to record the driver's boot-timeout strike after exit 80. The bench never escalates for this driver, and the error keeps the transient sentinel. fields step=record_boot_strike, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeLaunchErrorPersistFailed, "the captured launch stderr failed to persist as <workspace>/<agent>-launch-error.txt after a non-zero exit. The classified error stays unchanged and BRIDGE_EXIT_* drops launch_error. fields step=persist_launch_error, path, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeResultReadFailed, "the launch exited 0, but its result (artifact or stdout scrollback) failed to read into the response. Launch returns nil with empty Stdout, and the on-disk report holds the verdict. fields step=read_result, path, completion, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeFreshSessionRetry, "a launch runs as the ONE fresh session of the same CLI started by core.LaunchRecoverable after dead_shell or cli_self_updated. The dead row is marked session_recoverable and this row fresh_session_retry. Never set for named sessions, delivered runs, cancelled contexts, or tight deadlines (F31). Fields: call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeCompletedOnWorktreeEvidence, "a correction re-dispatch of a source-writing phase went idle without rewriting its deliverable after changing the worktree. Host state under .evolve/, the workspace, and deliverable paths never count. The bridge completed on the carried deliverable for verify and review to judge. Fields: deliverable, changed_paths, paths (at most eight, then +N more), cli")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneStagnant, "a tmux pane is busy but its output stopped changing (LivenessCenter edge: busy-stagnant)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneHung, "a tmux pane is hung: no progress and no completion (LivenessCenter edge: hung)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePhaseStalled, "the per-phase observer saw no progress for the stall threshold between review checkpoints. Tmux checks pane-watch hash (kind pane.liveness); headless checks stdout and workspace growth (kind observer.warning). Log and signal only; nothing is killed. Fields: source (pane | stdout), session, busy, stall_s")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneExhausted, "a tmux pane shows the CLI's quota/rate-limit exhaustion (LivenessCenter edge: exhausted; the exhaustion gate corroborates before rc 85)")
}

type dispatchIdentity struct {
	cycle int
	runID string
	phase string
}

func requestIdentity(req core.BridgeRequest) dispatchIdentity {
	return dispatchIdentity{cycle: req.Cycle, runID: req.RunID, phase: req.Agent}
}

func configIdentity(cfg *Config) dispatchIdentity {
	return dispatchIdentity{cycle: cfg.Cycle, runID: cfg.RunID, phase: cfg.Agent}
}

func paneLivenessHandler(signals *signalcenter.Center, id dispatchIdentity) panestream.LivenessHandler {
	return func(ev panestream.LivenessEvent) {
		state := ev.State.String()
		e := signalcenter.Event{
			Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleLiveness, Origin: "paneLivenessHandler",
			Kind: signalcenter.KindPaneLiveness, Severity: signalcenter.SeverityInfo,
			Reason: "pane " + ev.SessionKey + " is " + state,
			Fields: map[string]string{"session": ev.SessionKey, "state": state},
		}
		if code := paneLivenessCode(ev.State); code != "" {
			e.Severity, e.Code = signalcenter.SeverityWarn, code
		}
		signals.Emit(e)
	}
}

func paneLivenessCode(s panestream.LivenessState) signalcenter.Code {
	switch s {
	case panestream.LivenessBusyButStagnant:
		return CodePaneStagnant
	case panestream.LivenessHung:
		return CodePaneHung
	case panestream.LivenessExhausted:
		return CodePaneExhausted
	}
	return ""
}
