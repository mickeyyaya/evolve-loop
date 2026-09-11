package bridge

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// replWaiter owns the completion wait state machine after prompt dispatch. Its
// dependencies are fixed for one launch, which keeps the state transitions
// testable without widening the package API.
type replWaiter struct {
	ctx            context.Context
	cfg            *Config
	deps           Deps
	launch         tmuxLaunch
	prefix         string
	phaseName      string
	resolvedPrompt string
	artifactBase   artifactBaseline
	responder      *autoResponder
	recorder       *interaction.Recorder
	cursor         *inbox.Cursor
	channel        *replLiveChannel
}

func newReplInboxCursor(cfg *Config) *inbox.Cursor {
	cursor := inbox.NewCursor(cfg.Workspace, cfg.Agent)
	if fi, err := os.Stat(inbox.Path(cfg.Workspace, cfg.Agent)); err == nil {
		cursor.SetOffset(fi.Size())
	}
	return cursor
}

func (w replWaiter) wait() (replWaitResult, int) {
	ctx := w.ctx
	cfg := w.cfg
	deps := w.deps
	lp := w.launch
	pfx := w.prefix
	phaseName := w.phaseName
	ar := w.responder
	irec := w.recorder
	channel := w.channel
	state := newReplWaitState(w)
	defer state.recordNudgeOutcome(irec, deps.Now)

	if code := w.admitPrompt(state); code != ExitOK {
		return state.result, code
	}
	ar.transientDwellEnabled = true
	for elapsed := 0; ; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		state.waitedS = elapsed
		if err := ctx.Err(); err != nil {
			// Context cancelled (orchestrator timeout / SIGTERM / the next phase
			// tearing down this session): before abandoning, ONE final completion
			// poll — a deliverable already on disk means the session COMPLETED and
			// the cancel is benign teardown, not a phase timeout. Pre-fix this
			// break skipped straight to the !completed → ExitArtifactTimeout exit,
			// laundering a finished session into a timeout (session-lifecycle
			// residual; the runner's settle-retry was the only thing standing
			// between that mislabel and a false FAIL). A genuinely unfinished
			// session still exits ExitArtifactTimeout.
			//
			// The poll gets a context DETACHED from the cancellation (cycle-1236):
			// this line dispatches to all three completionDetector strategies, and
			// only artifactDetector is a pure file stat. stdoutDetector shells
			// CapturePane and gitEvidenceDetector shells git — exec.CommandContext
			// refuses to fork on a dead ctx, both swallow the transport error as
			// "not ready", and a DELIVERED stdout/git phase exited 81. withFinalPoll
			// hands them a live, finalPollGrace-bounded context carrying the
			// explicit finality marker artifactDetector's short-circuit now keys on
			// (a live ctx alone would have disarmed it — completion.go).
			finalCtx, finalCancel := withFinalPoll(ctx)
			ready, _, note, _ := state.detector.poll(finalCtx)
			finalCancel()
			if ready {
				state.completed = true
				if note != "" {
					fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
				}
				fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) AFTER completion — benign teardown of a finished session\n", pfx, err)
				break
			}
			// Load-bearing once a Stage-1 LLM reviewer can extend at length:
			// stop waiting promptly rather than running out the extend budget.
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning completion wait\n", pfx, err)
			break
		}
		// Live channel: stream newly-stabilized rendered content to pane.live.
		// The first Next() primes the baseline (echoed prompt + boot chrome are
		// counted, not emitted); later ticks emit only the assistant output that
		// appeared above the volatile input box. Gated so off adds no capture.
		var waitPane string
		channelCaptureOK := true
		if channel.on {
			// This is the canonical pane observation for channel-enabled waits;
			// taking another in auto-response can skip alternating dwell frames.
			var captureErr error
			waitPane, captureErr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			channelCaptureOK = captureErr == nil
			if captureErr == nil {
				state.result.recordTokens(waitPane)
				channel.streamPane(waitPane)
			}
		}
		ready, _, note, derr := state.detector.poll(ctx)
		if ready {
			state.completed = true
			if note != "" {
				fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
			}
			break
		}
		if derr != nil && !state.detectorErrorLogged {
			// The detector surfaced a fault (e.g. an artifact present at a
			// non-canonical path that could not be relocated — read-only
			// workspace). Surface it once, immediately, instead of spinning the
			// full wait window with no signal.
			fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, derr)
			state.detectorErrorLogged = true
		}
		waitCaptureOK := true
		if !channel.on {
			// Preserve the legacy ordering: completed artifact waits break above
			// without consuming a pane frame solely for auto-response.
			var werr error
			waitPane, werr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			waitCaptureOK = werr == nil
		}
		step := w.handleTickInteractions(state, elapsed, waitPane, channelCaptureOK && waitCaptureOK)
		if step.done {
			if step.code != ExitOK {
				return state.result, step.code
			}
			break
		}
		// Review checkpoint: a full interval elapsed without the artifact.
		if elapsed-state.intervalStartS >= state.intervalS {
			rawPane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			curPane, renderWedged := recoverBlankPane(ctx, deps, lp.session, lp.bootScrollback, rawPane, pfx)
			if w.reviewCheckpoint(state, elapsed, curPane, renderWedged) == checkpointFailover {
				return state.result, ExitUnknownPrompt
			}
			if state.lastVerdict.Action != ReviewExtend {
				_, isDetVal := state.reviewer.(deterministicReviewer)
				_, isDetPtr := state.reviewer.(*deterministicReviewer)
				isDeterministic := isDetVal || isDetPtr
				// Nudge only on PAUSE (idle agent, remind it once). A fatal
				// ReviewStop (ADR-0044 C2) must exit now — nudging a dead
				// shell is exactly the echo that bought cycle-262's dead
				// panes their extensions. Behavior-identical for the legacy
				// reviewer, which only ever emits extend|pause.
				if state.lastVerdict.Action == ReviewPause && isDeterministic && !state.livenessCenter.Busy(lp.session) && !state.nudgeSent {
					nudgeMsg := fmt.Sprintf("Please write the deliverable to %s to complete the phase.", cfg.Artifact)
					_ = deps.Tmux.SendKeys(ctx, lp.session, nudgeMsg, true)
					// The recorded stall (cycles 1505/1510/1517): this nudge was
					// still parked at the `❯` input line in the final capture and
					// every interaction record read result=no_effect. Verify it
					// was submitted; re-send Enter, bounded, if it was not.
					// A fresh capture is unavoidable here: the nudge was sent
					// microseconds ago, so every earlier pane predates it.
					// Announce the nudge BEFORE verifying it, so an operator never reads
					// "re-sending Enter" above any line saying a nudge exists.
					fmt.Fprintf(deps.Stderr, "%s idle with missing artifact; sent one-shot nudge: %s\n", pfx, nudgeMsg)
					deps.Sleep(submitVerifySettle)
					nudgePane, nudgeCapErr := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
					if nudgeCapErr != nil {
						fmt.Fprintf(deps.Stderr, "%s submit-verify: nudge NOT verified — capture failed, input-line state unknown: %v\n", pfx, nudgeCapErr)
					}
					nudgeOutcome := verifySubmitted(ctx, deps, lp, pfx, "nudge", nudgePane, nudgeMsg)
					recordSubmitVerify(irec, phaseName, cfg.Cycle, "nudge", nudgeOutcome)
					if nudgeOutcome.Result == interaction.ResultSubmitWedged {
						state.lastVerdict.Reason = fmt.Sprintf("nudge %s (resends=%d)", nudgeOutcome.Result, nudgeOutcome.Resends)
						break
					}
					state.nudgeSent = true
					state.nudgeEvent = &interaction.Event{
						Kind:    interaction.KindNudge,
						Phase:   phaseName,
						Cycle:   cfg.Cycle,
						Trigger: "idle_no_artifact",
						Payload: nudgeMsg,
					}
					state.nudgeAt = deps.Now()
					state.intervalStartS = elapsed
					state.attempt++
					continue
				}
				break
			}
			state.attempt++
			state.intervalStartS = elapsed
		}
	}
	if !state.completed {
		fmt.Fprintf(deps.Stderr, "%s FAIL: completion never signalled (artifact %s; stop-review paused after %d interval(s) of %ds)\n", pfx, cfg.Artifact, state.attempt+1, state.intervalS)
		fmt.Fprintf(deps.Stderr, "%s diagnostic: files present under workspace %s:\n", pfx, cfg.Workspace)
		for _, line := range listWorkspaceFiles(cfg.Workspace) {
			fmt.Fprintf(deps.Stderr, "%s   %s\n", pfx, line)
		}
		// Pause (ambiguous stall) and Stop (typed fatal fast-fail, ADR-0044
		// C2) both leave the operator-facing escalation report; extend keeps
		// the legacy no-report behavior.
		if state.lastVerdict.Action == ReviewPause || state.lastVerdict.Action == ReviewStop {
			_ = writeEscalationReport(cfg.Workspace, phaseName, cfg.Cycle, state.lastEvent, state.lastVerdict)
		}
		// Fail-loud drift alarm (exhaustion_drift.go): if this timed-out pane looks
		// like a quota wall the exhausted_regex missed, the wall wording may have
		// drifted ahead of the pattern — surface it now so the NEXT drift is caught
		// in one cycle, not eight (as the per-model wording change was). Diagnostic
		// only; the exit-81 verdict stands.
		// Scanned on the SAME agent-stripped pane as the primary detector above:
		// an alarm that scans the raw pane
		// fires on wall-shaped text the agent merely wrote or echoed, which the real
		// regex correctly ignored — an operator chasing a regex that is working.
		strippedPane := strippedForExhaustionScan(state.result.lastGoodPane, ar.injectedPrompt)
		warnExhaustionRegexDrift(deps.Stderr, pfx, lp.name, strippedPane, state.paneProfile.ExhaustedRegex)
		// Transient-upstream recognition (inbox item
		// transient-api-error-invisible-inside-artifact-timeout): 3 of 4 observed
		// router stalls (cycles 1523/1524/1526) spent the FULL silence budget on a
		// pane reading "API Error: 529 Overloaded … usually temporary" — a failure
		// no path recognizes. It is not a quota wall (exhausted_regex correctly
		// ignores it) and not a transient exit code (80/85/86). The distinguishing
		// evidence was already captured right here and simply never consulted.
		//
		// Family-agnostic by construction: the pattern is resolved from the
		// LAUNCHED cli's manifest (lp.name), so every CLI family declares its own
		// provider's transient signature and none of that vocabulary is hard-coded
		// here. A family that declares none is fail-open, not misreported.
		//
		// Carried as ONE driver-authored field on the marker line below, NOT as a
		// second cause candidate: extending that line means it can never displace
		// artifactTimeoutSummary's selection, so the class of regression where the
		// drift alarm or a workspace listing outranks the timeout summary stays
		// structurally unreachable. The exit code is untouched — 81 remains
		// non-transient (transient-bridge-retry AC-1) and the discrimination rides
		// the cause as data. A bool, never pane text, also keeps agent-authored
		// content out of the recorded cause (F1 indirect-prompt-injection).
		//
		// Scanned on the SAME agent-stripped pane as the drift alarm above: a raw
		// scan fires on error text the agent merely quoted, flagging a working
		// agent's own prose as an upstream outage.
		transient := classifyTransientPane(lp.name, strippedPane)
		// Self-describing death (inbox item deep-phase-artifact-budget-too-small):
		// exit 81 alone says nothing, and the diagnostic file listing printed above
		// used to become the recorded cause. This ONE marker line carries how long
		// the wait ran and how much of the extend budget it consumed, so the next
		// reader can separate "too slow — raise bridge.phase_artifact_timeout_s for
		// this phase" (extends_used == max_extends while the agent was busy) from
		// "wedged" (paused early, idle, no progress). The phase= field is
		// cfg.Agent — the SAME key bridge.phase_artifact_timeout_s is indexed on —
		// so the remedy is copy-pasteable from the diagnostic. Emitted LAST so it
		// is also the final stderr line, and lifted into the error by Engine.Launch.
		state.writeArtifactTimeoutMarker(deps.Stderr, phaseName, transient)
		if state.transientShortcircuit && ctx.Err() == nil {
			// Deliberately a plain blocking pause: it throttles re-dispatch
			// against a provider that just spent 60s saying "overloaded", and
			// under cliadmit it holds this family's slot for the cooldown —
			// which IS the point during a provider-wide 529 storm. The ctx
			// guard skips it when the run is already cancelled; a mid-sleep
			// cancel waits out at most 15s, bounded and small against the
			// budget this path just saved.
			deps.Sleep(transientRedispatchDelay)
		}
		return state.result, ExitArtifactTimeout
	}

	return state.result, ExitOK
}
