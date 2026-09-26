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
	artifactBase   artifactBaseline
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
	defer state.recordNudgeOutcome(irec, deps.Now)

	if code := w.admitPrompt(state); code != ExitOK {
		return state.result, code
	}
	ar.transientDwellEnabled = true
	for elapsed := 0; ; elapsed += 2 {
		deps.Sleep(artifactWaitInterval)
		state.waitedS = elapsed
		if err := ctx.Err(); err != nil {
			// One final completion poll before abandoning: a deliverable already
			// on disk means the cancel is benign teardown, not a timeout.
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
			// Load-bearing once a Stage-1 LLM reviewer can extend at length:
			// stop waiting promptly rather than running out the extend budget.
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning completion wait\n", pfx, err)
			state.cancellationErr = err
			break
		}
		// Live channel: stream newly-stabilized rendered content to pane.live;
		// gated so off adds no capture.
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
		logDetectorError := state.observeDetector(derr)
		if ready {
			state.completed = true
			if note != "" {
				fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
			}
			break
		}
		if logDetectorError {
			// The detector surfaced a fault (e.g. an artifact present at a
			// non-canonical path that could not be relocated — read-only
			// workspace). Surface it once, immediately, instead of spinning the
			// full wait window with no signal.
			fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, derr)
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
			if w.applyCheckpointDisposition(state, elapsed) == checkpointStopWaiting {
				break
			}
		}
	}
	return w.closeout(state)
}
