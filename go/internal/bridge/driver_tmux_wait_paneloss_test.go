package bridge

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
)

type serverLossTmux struct {
	fakeTmux
	serverGone   bool
	captureFails bool
	blips        int
	hasSessions  int
	goneCaptures int
	cancelAt     int
	cancel       context.CancelFunc
}

func (s *serverLossTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	if s.serverGone {
		s.goneCaptures++
		if s.goneCaptures == s.cancelAt {
			s.cancel()
		}
	}
	if s.serverGone || s.captureFails || s.blips > 0 {
		return "", errors.New("tmux capture-pane: exit status 1: no server running on /private/tmp/tmux-501/evolve-bridge-p21421")
	}
	return s.fakeTmux.CapturePane(ctx, session, scrollback)
}

func (s *serverLossTmux) HasSession(ctx context.Context, name string) bool {
	s.hasSessions++
	if ctx.Err() != nil {
		return false
	}
	if s.blips > 0 {
		s.blips--
		return false
	}
	return !s.serverGone && s.fakeTmux.HasSession(ctx, name)
}

type paneLossRun struct {
	code          int
	stderr        string
	waitTicks     int
	lossTick      int
	hasSessionsAt []int
}

func dispatchUntilPaneLoss(t *testing.T, tm *serverLossTmux, lose func(*serverLossTmux)) paneLossRun {
	t.Helper()
	return dispatchUntilPaneLossIn(t, context.Background(), tm, lose)
}

func dispatchUntilPaneLossIn(t *testing.T, ctx context.Context, tm *serverLossTmux, lose func(*serverLossTmux)) paneLossRun {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	tm.paneSeq = []string{tmuxPromptMarkerDefault}
	run := paneLossRun{lossTick: 3}
	sleep := func(delay time.Duration) {
		if delay != artifactWaitInterval {
			return
		}
		run.waitTicks++
		run.hasSessionsAt = append(run.hasSessionsAt, tm.hasSessions)
		if run.waitTicks == run.lossTick {
			lose(tm)
		}
	}
	eng := newTestEngine(Deps{
		Tmux:               tm,
		Sleep:              sleep,
		ArtifactTimeoutS:   1200,
		ArtifactMaxExtends: 1,
		CaptureBaseline:    zeroBaselineCapture,
	})
	var stdout, stderr bytes.Buffer
	run.code = eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)
	run.stderr = stderr.String()
	return run
}

func TestRunTmuxREPL_ATmuxServerKilledMidPhaseEndsTheDispatchAsPaneLostWithinOneLivenessInterval(t *testing.T) {
	tm := &serverLossTmux{}

	run := dispatchUntilPaneLoss(t, tm, func(s *serverLossTmux) { s.serverGone = true })

	if run.code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", run.code, ExitArtifactTimeout, run.stderr)
	}
	if got := launchoutcome.CauseCode(run.code, run.stderr); got != string(launchoutcome.TimeoutPaneLost) {
		t.Errorf("cause_code = %q, want %q: a lost pane is its own classified outcome, never a stall review", got, launchoutcome.TimeoutPaneLost)
	}
	if limit := run.lossTick + paneLossConfirmTicks; run.waitTicks > limit {
		t.Errorf("wait ticks = %d, want <= %d: the loss is confirmed within one liveness interval, not at the %ds review checkpoint", run.waitTicks, limit, 1200)
	}
	summary := launchoutcome.ArtifactTimeoutSummary(run.stderr)
	if !strings.Contains(summary, "busy=false") || strings.Contains(summary, "last_review=pause") {
		t.Errorf("summary = %q, want a lost pane that is not busy and was not paused by a stall review", summary)
	}
}

func TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch(t *testing.T) {
	tm := &serverLossTmux{}

	run := dispatchUntilPaneLoss(t, tm, func(s *serverLossTmux) { s.captureFails = true })

	if got := launchoutcome.CauseCode(run.code, run.stderr); got == string(launchoutcome.TimeoutPaneLost) {
		t.Fatalf("cause_code = %q: has-session still finds the session, so a capture failure alone is not a lost pane", got)
	}
	if run.waitTicks < 1200/2 {
		t.Errorf("wait ticks = %d, want the full review interval: a live session keeps its dispatch", run.waitTicks)
	}
}

func TestRunTmuxREPL_OneTickWithoutTheSessionIsNotYetAPaneLoss(t *testing.T) {
	tm := &serverLossTmux{}

	run := dispatchUntilPaneLoss(t, tm, func(s *serverLossTmux) { s.blips = 1 })

	if got := launchoutcome.CauseCode(run.code, run.stderr); got == string(launchoutcome.TimeoutPaneLost) {
		t.Fatalf("cause_code = %q: one failed has-session is a blip; the loss needs %d consecutive ticks", got, paneLossConfirmTicks)
	}
	if run.waitTicks < 1200/2 {
		t.Errorf("wait ticks = %d, want the full review interval after a one-tick blip", run.waitTicks)
	}
}

func TestRunTmuxREPL_ACancelDuringTheLossConfirmationEndsAsContextCancelledNeverPaneLost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tm := &serverLossTmux{cancelAt: paneLossConfirmTicks, cancel: cancel}

	run := dispatchUntilPaneLossIn(t, ctx, tm, func(s *serverLossTmux) { s.serverGone = true })

	if got := launchoutcome.CauseCode(run.code, run.stderr); got != string(launchoutcome.TimeoutContextCancelled) {
		t.Fatalf("cause_code = %q, want %q: a cancel that lands while the loss is being confirmed is our own teardown, not a lost pane; stderr:\n%s", got, launchoutcome.TimeoutContextCancelled, run.stderr)
	}
}

func TestRunTmuxREPL_AHealthyCaptureNeverQueriesTheSession(t *testing.T) {
	tm := &serverLossTmux{}

	run := dispatchUntilPaneLoss(t, tm, func(*serverLossTmux) {})

	if run.code != ExitArtifactTimeout || len(run.hasSessionsAt) < run.lossTick {
		t.Fatalf("exit = %d with %d wait ticks, want %d after at least %d ticks", run.code, len(run.hasSessionsAt), ExitArtifactTimeout, run.lossTick)
	}
	if delta := run.hasSessionsAt[run.lossTick-1] - run.hasSessionsAt[0]; delta != 0 {
		t.Errorf("has-session calls across %d healthy ticks = %d, want 0: the loss check runs only after a capture fails", run.lossTick-1, delta)
	}
}
