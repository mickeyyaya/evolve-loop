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

// artifactWaitInterval is the artifact-wait poll cadence.
const artifactWaitInterval = 2 * time.Second

type replWaiter struct {
	ctx            context.Context
	cfg            *Config
	deps           Deps
	launch         tmuxLaunch
	prefix         string
	phaseName      string
	resolvedPrompt string
	dispatchBase   dispatchBaseline
	paste          pasteOutcome // what the prompt delivery learned — recorded beside the submit verdict
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
	deps := w.deps
	lp := w.launch
	pfx := w.prefix
	ar := w.responder
	irec := w.recorder
	channel := w.channel
	state := newReplWaitState(w)
	defer state.finishWait(irec, deps.Now)

	if code := w.admitPrompt(state); code != ExitOK {
		return state.result, code
	}
	ar.transientDwellEnabled = true
	for elapsed := 0; ; elapsed += 2 {
		w.pace()
		state.waitedS = elapsed
		if err := ctx.Err(); err != nil {
			finalCtx, finalCancel := withFinalPoll(ctx)
			ready, _, note, detectorErr := state.detector.poll(finalCtx)
			finalCancel()
			logDetectorError := state.observeDetector(detectorErr)
			if ready {
				state.completed = true
				if note != "" {
					fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
				}
				fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) AFTER completion — benign teardown of a finished session\n", pfx, err)
				break
			}
			if logDetectorError {
				fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, detectorErr)
			}
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning completion wait\n", pfx, err)
			state.cancellationErr = err
			break
		}
		var waitPane string
		channelCaptureOK := true
		if channel.on {
			var captureErr error
			waitPane, captureErr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			channelCaptureOK = captureErr == nil
			if captureErr == nil {
				state.result.recordTokens(waitPane)
				channel.streamPane(waitPane)
			}
		}
		ready, _, note, derr := state.detector.poll(ctx)
		logDetectorError := state.observeDetector(derr)
		if ready {
			state.completed = true
			if note != "" {
				fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
			}
			break
		}
		if logDetectorError {
			fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, derr)
		}
		waitCaptureOK := true
		if !channel.on {
			var werr error
			waitPane, werr = deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			waitCaptureOK = werr == nil
		}
		if w.paneLost(state, channelCaptureOK && waitCaptureOK) {
			break
		}
		step := w.handleTickInteractions(state, elapsed, waitPane, channelCaptureOK && waitCaptureOK)
		if step.done {
			if step.code != ExitOK {
				return state.result, step.code
			}
			break
		}
		if elapsed-state.intervalStartS >= state.intervalS {
			rawPane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			curPane, renderWedged := recoverBlankPane(ctx, deps, lp.session, lp.bootScrollback, rawPane, pfx)
			if w.reviewCheckpoint(state, elapsed, curPane, renderWedged) == checkpointFailover {
				return state.result, ExitUnknownPrompt
			}
			if w.applyCheckpointDisposition(state, elapsed) == checkpointStopWaiting {
				break
			}
		}
	}
	return w.closeout(state)
}
