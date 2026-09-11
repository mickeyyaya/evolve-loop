package bridge

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type reviewBeforeNudgeTmux struct {
	*fakeTmux
	artifact          string
	reviewObserved    *bool
	nudgeBeforeReview bool
}

func (t *reviewBeforeNudgeTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if strings.Contains(keys, t.artifact) && !*t.reviewObserved {
		t.nudgeBeforeReview = true
	}
	return t.fakeTmux.SendKeys(ctx, session, keys, enter)
}

// TestRunTmuxREPL_StopReviewCallbackPrecedesNudge characterizes the seam
// between checkpoint adjudication and disposition. Consumers must observe the
// pause verdict before the driver acts on it by nudging the pane.
func TestRunTmuxREPL_StopReviewCallbackPrecedesNudge(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	reviewObserved := false
	tmux := &reviewBeforeNudgeTmux{
		fakeTmux:       &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		artifact:       fx.artifact,
		reviewObserved: &reviewObserved,
	}
	deps := Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		LookupEnv:        mapLookup(nil),
		CaptureBaseline:  zeroBaselineCapture,
		ArtifactTimeoutS: 2,
		RecoveryStage:    "off",
		OnStopReview: func(_ string, action string, _ string) {
			if action == string(ReviewPause) {
				reviewObserved = true
			}
		},
	}
	eng := newTestEngine(deps)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr.String())
	}
	if !tmux.sentContains(fx.artifact) {
		t.Fatal("precondition: idle checkpoint did not send the artifact nudge")
	}
	if !reviewObserved {
		t.Fatal("pause review callback was not published")
	}
	if tmux.nudgeBeforeReview {
		t.Error("artifact nudge was sent before the pause review callback")
	}
}

func TestApplyCheckpointDisposition_ExtendAdvancesInterval(t *testing.T) {
	state := &replWaitState{
		lastVerdict:    ReviewVerdict{Action: ReviewExtend, Reason: "still working"},
		attempt:        2,
		intervalStartS: 4,
	}

	if got := (replWaiter{}).applyCheckpointDisposition(state, 10); got != checkpointContinueWaiting {
		t.Fatalf("disposition = %v, want checkpointContinueWaiting", got)
	}
	if state.attempt != 3 || state.intervalStartS != 10 {
		t.Errorf("advanced state = attempt %d, interval start %d; want 3 and 10",
			state.attempt, state.intervalStartS)
	}
}

func newDispositionFixture(t *testing.T, action ReviewAction) (replWaiter, *replWaitState, *fakeTmux) {
	t.Helper()
	workspace := t.TempDir()
	session := "disposition-unit"
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	deps := Deps{
		Tmux:          tmux,
		Sleep:         func(time.Duration) {},
		Now:           func() time.Time { return time.Unix(1_700_000_000, 0) },
		Stderr:        &bytes.Buffer{},
		RecoveryStage: "off",
	}.withDefaults()
	center := panestream.NewSignalCenter()
	center.Observe(session, tmuxPromptMarkerDefault, panestream.Profiles["claude"])
	waiter := replWaiter{
		ctx: context.Background(),
		cfg: &Config{
			Agent:     "build",
			Cycle:     7,
			Workspace: workspace,
			Artifact:  filepath.Join(workspace, "build-report.md"),
		},
		deps:      deps,
		launch:    tmuxLaunch{name: "claude-tmux", session: session, inputLineMarker: tmuxPromptMarkerDefault},
		prefix:    "[disposition-unit]",
		phaseName: "build",
		recorder:  interaction.NewRecorder(workspace),
	}
	state := &replWaitState{
		lastVerdict:    ReviewVerdict{Action: action, Reason: "unit verdict"},
		reviewer:       newDeterministicReviewer(defaultArtifactMaxExtends),
		livenessCenter: center,
		attempt:        2,
		intervalStartS: 4,
	}
	return waiter, state, tmux
}

func TestApplyCheckpointDisposition_StopActionsDoNotNudge(t *testing.T) {
	for _, action := range []ReviewAction{ReviewStop, "", "unexpected"} {
		t.Run(string(action), func(t *testing.T) {
			waiter, state, tmux := newDispositionFixture(t, action)

			if got := waiter.applyCheckpointDisposition(state, 10); got != checkpointStopWaiting {
				t.Fatalf("disposition = %v, want checkpointStopWaiting", got)
			}
			if len(tmux.sentKeys) != 0 || len(tmux.captureScrollback) != 0 {
				t.Errorf("stop action performed pane I/O: sends=%v captures=%v",
					tmux.sentKeys, tmux.captureScrollback)
			}
			if state.attempt != 2 || state.intervalStartS != 4 {
				t.Errorf("stop action mutated interval state: attempt %d, interval start %d",
					state.attempt, state.intervalStartS)
			}
		})
	}
}

func TestApplyCheckpointDisposition_IdlePauseNudgesAndAdvancesOneInterval(t *testing.T) {
	waiter, state, tmux := newDispositionFixture(t, ReviewPause)

	if got := waiter.applyCheckpointDisposition(state, 10); got != checkpointContinueWaiting {
		t.Fatalf("disposition = %v, want checkpointContinueWaiting", got)
	}
	if len(tmux.sentKeys) != 1 || !strings.Contains(tmux.sentKeys[0], waiter.cfg.Artifact) {
		t.Fatalf("nudge sends = %v, want one message naming %s", tmux.sentKeys, waiter.cfg.Artifact)
	}
	if len(tmux.captureScrollback) != 1 {
		t.Errorf("verification captures = %d, want 1", len(tmux.captureScrollback))
	}
	if state.attempt != 3 || state.intervalStartS != 10 {
		t.Errorf("nudge interval state = attempt %d, interval start %d; want 3 and 10",
			state.attempt, state.intervalStartS)
	}
	if !state.nudgeSent || state.nudgeEvent == nil {
		t.Fatalf("nudge state was not retained: sent=%v event=%+v", state.nudgeSent, state.nudgeEvent)
	}
}

func TestApplyCheckpointDisposition_WedgedNudgeDoesNotAdvanceInterval(t *testing.T) {
	waiter, state, baseTmux := newDispositionFixture(t, ReviewPause)
	waiter.deps.Tmux = &stickyInputTmux{
		fakeTmux:      baseTmux,
		trigger:       waiter.cfg.Artifact,
		clearOnResend: false,
	}

	if got := waiter.applyCheckpointDisposition(state, 10); got != checkpointStopWaiting {
		t.Fatalf("disposition = %v, want checkpointStopWaiting", got)
	}
	if state.attempt != 2 || state.intervalStartS != 4 {
		t.Errorf("wedged nudge advanced interval state: attempt %d, interval start %d",
			state.attempt, state.intervalStartS)
	}
	if state.nudgeSent || state.nudgeEvent != nil {
		t.Errorf("wedged nudge armed deferred outcome: sent=%v event=%+v", state.nudgeSent, state.nudgeEvent)
	}
	if !strings.Contains(state.lastVerdict.Reason, interaction.ResultSubmitWedged) {
		t.Errorf("wedged reason = %q, want %q", state.lastVerdict.Reason, interaction.ResultSubmitWedged)
	}
}
