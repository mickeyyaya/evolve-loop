package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
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
			state.result.recordTokens(curPane)
			// CB.6: evidence survives the session's death. When the server
			// is killed mid-phase every later capture is empty, so the
			// escalation report's final_pane carried nothing and cycle-286's
			// retro misattributed the failure. Retain the last NON-EMPTY
			// pane; a dead capture falls back to it as the freshest real
			// evidence (the live pane still wins whenever it renders).
			if strings.TrimSpace(curPane) != "" {
				state.result.lastGoodPane = curPane
			}
			evidencePane := state.result.lastGoodPane
			// Progressed = the pane changed during the interval. Stage-0 signal:
			// good for the common cases (growing token counters, new tool calls),
			// but a pure spinner/clock animation also reads as progress — so the
			// maxExtends backstop, not this diff, bounds a spinner-stuck agent
			// (~maxExtends×interval). Stage 1's reviewer inspects StdoutTail to
			// disambiguate genuine work from animation.
			// Liveness WITHOUT the exhaustion override (empty ExhaustedRegex): a
			// wall in the pane must not collapse the real converging/idle signal,
			// because the exhaustion decision is made SEPARATELY below — on the
			// stripped pane and persistence-gated — so wall-shaped text a working
			// agent merely rendered cannot pause or kill it. Safe: nothing
			// consumes the center's Exhausted SignalEvent (no RegisterSignalHandler
			// caller); this fast-fail was its only actor.
			state.livenessCenter.Observe(lp.session, curPane, state.livenessProfile)
			livenessState := state.livenessCenter.Aggregate()
			// Exhaustion fast-fail: a quota/rate-limit WALL means the artifact will
			// NEVER come — fail over to the fallback CLI (exit 85) instead of
			// burning the full artifact timeout. Scanned on the agent-stripped pane
			// and persistence-gated (exhaustion_persistence.go): a re-printing real wall is
			// present every checkpoint and still crosses (preserving the agy
			// hang-without-exit fix), while wall text passing through a WORKING
			// agent's pane clears by the next checkpoint — the raw-pane false-FAIL
			// class the go-review of the per-model regex fix surfaced.
			walled := state.livenessCenter.ExhaustedOf(strippedForExhaustionScan(curPane, ar.injectedPrompt), state.paneProfile)
			// Corroboration contract shared with the fast-poll site, one
			// probe per phase, latched (wallcorroborate.go checkpointWallState).
			if escalate, suppressNow := state.checkpointWall.decide(ctx, deps.CorroborateWall, lp.name, state.checkpointExhaustion.observe(walled)); escalate {
				fmt.Fprintf(deps.Stderr, "%s EXHAUSTED: pane shows a quota/rate-limit wall (persisted %d checkpoints, corroborated by live probe) — failing over to fallback CLI (exit %d)\n", pfx, exhaustionPersistObservations, ExitUnknownPrompt)
				return state.result, ExitUnknownPrompt
			} else if suppressNow {
				fmt.Fprintf(deps.Stderr, "%s EXHAUSTION-SUPPRESSED: pane matched wall vocabulary but a live probe (cheapest tier) answered — treating as content-induced; a tier-scoped wall would surface via artifact-timeout fallback instead\n", pfx)
			}
			// Progressed is sourced from the center's Changed(session)
			// projection (S4) — a consecutive-observation comparison, not the
			// interval-baseline diff. Behavior-preserving: .Progressed has no
			// decision consumer (evidence/logging only), so the shift from
			// baseline-relative to checkpoint-to-checkpoint is safe.
			progressed := state.livenessCenter.Changed(lp.session)
			// Render-wedge override (cycle-291): a blank pane from a live session
			// reads as Idle by the content-velocity detector (no affordance in blank
			// frame). recoverBlankPane already confirmed the session is alive
			// (renderWedged=true); treat as BusyButStagnant so the reviewer extends
			// rather than pausing a working agent on a pane-rendering failure.
			if renderWedged && livenessState == panestream.LivenessIdle {
				livenessState = panestream.LivenessBusyButStagnant
			}
			state.lastEvent = StopEvent{
				Kind:       StopArtifactTimeout,
				Phase:      cfg.Agent,
				Cycle:      cfg.Cycle,
				ElapsedS:   elapsed,
				IntervalS:  state.intervalS,
				Attempt:    state.attempt,
				Progressed: progressed,
				Busy:       state.livenessCenter.Busy(lp.session) || renderWedged,
				StdoutTail: lastLines(evidencePane, 40),
				// Same source the exhaustion scan strips against, two lines up —
				// one resolved prompt, one meaning of "what the agent was told",
				// so the two detectors can never strip differently (cycle-1117).
				InjectedPrompt: ar.injectedPrompt,
				State:          livenessState,
			}
			// ADR-0044 C2: a known-fatal pane (model-invalid boot, CLI
			// self-update, dead shell) preempts the reviewer in enforce —
			// cycle-262's dead panes read as "progressed" because the
			// bridge's own nudge echoed into them, so the legacy
			// extend-while-progressing flow burned the full maxExtends
			// backstop on REPLs that no longer existed.
			// Persistence-gated (fatalpane_persistence.go): the match must be
			// present on consecutive checkpoints before it can preempt, so a
			// working agent that renders fatal-shaped text for one frame is
			// never killed for it, while a parked pane still exits one
			// checkpoint later than before.
			// The gate RECORDS a C2 evidence outcome on every gate-crossed
			// call (R8.3) — it must be called exactly once per stop-review
			// checkpoint, never retried for the same event, or the soak's C2
			// counts inflate silently.
			v, preempted := state.checkpointFatal.verdict(state.fatalDetector, state.lastEvent, state.recoveryStage, irec, deps.Stderr, pfx)
			if !preempted {
				v = state.reviewer.Review(state.lastEvent)
			}
			state.lastVerdict = v
			fmt.Fprintf(deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
				pfx, StopArtifactTimeout, elapsed, state.attempt, progressed, state.lastVerdict.Action, state.lastVerdict.Reason)
			if deps.OnStopReview != nil {
				deps.OnStopReview(phaseName, string(state.lastVerdict.Action), state.lastVerdict.Reason)
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
		// Scanned on the SAME agent-stripped pane as the primary detector above
		// (strippedForExhaustionScan, line ~704): an alarm that scans the raw pane
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
