package bridge

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

type serverGoneTmux struct {
	*FakeTmuxController
	sessionGone bool
}

func (s *serverGoneTmux) HasSession(ctx context.Context, name string) bool {
	return !s.sessionGone && s.FakeTmuxController.HasSession(ctx, name)
}

func (s *serverGoneTmux) CapturePane(context.Context, string, int) (string, error) {
	return "", errors.New("no server running on /tmp/tmux-501/evolve")
}

type sessionGoneFake struct{ *fakeTmux }

func (sessionGoneFake) HasSession(context.Context, string) bool { return false }

func observedBootLoss(t *testing.T, tm TmuxController, lp tmuxLaunch) launchoutcome.BootLossCause {
	t.Helper()
	var stderr bytes.Buffer
	var observed launchoutcome.BootLossCause
	deps := fixtureDeps(tm)
	deps.Stderr = &stderr
	deps.BootTimeoutS = 3
	deps.onBootLoss = func(cause launchoutcome.BootLossCause) { observed = cause }
	code, err := runTmuxREPL(context.Background(), fixtureConfig(t), deps, lp)
	if err != nil || code != ExitREPLBootTimeout {
		t.Fatalf("runTmuxREPL = (%d, %v), want ExitREPLBootTimeout\nstderr:\n%s", code, err, stderr.String())
	}
	return observed
}

func TestBootTimeout_ASessionGoneAtTheDeadlineReportsPaneLost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		gone bool
		want launchoutcome.BootLossCause
	}{
		{"the session is gone", true, launchoutcome.BootLossPaneLost},
		{"the session is alive", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tm := &serverGoneTmux{FakeTmuxController: &FakeTmuxController{}, sessionGone: tc.gone}

			got := observedBootLoss(t, tm, tmuxLaunch{
				name: "claude-tmux", session: "bootloss-" + string(tc.want), launchCmd: "claude",
				promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
			})

			if got != tc.want {
				t.Fatalf("observed %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPostPasteShellSpill_ReportsDeadShell(t *testing.T) {
	t.Parallel()
	base := &FakeTmuxController{CaptureFrames: []string{"❯", cycle274BquoteSpill, cycle274BquoteSpill}, PaneCmd: "node"}
	tm := &shellWedgeTmux{FakeTmuxController: base, postPasteCmd: "zsh"}

	got := observedBootLoss(t, tm, tmuxLaunch{
		name: "claude-tmux", session: "bootloss-spill", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, guardDeadShell: true,
	})

	if got != launchoutcome.BootLossDeadShell {
		t.Fatalf("observed %q, want dead_shell: the CLI process is gone after the paste", got)
	}
}

func TestReportBootLoss_WithNoObserverOnlyWritesTheDiagnostic(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	reportBootLoss(Deps{Stderr: &stderr}, launchoutcome.BootLossPaneLost, "gone")
	if stderr.Len() == 0 {
		t.Fatal("no diagnostic line")
	}
}

func TestEngineLaunch_ABootTimeoutOnALostSessionCarriesPaneLostToTheResponseAndTheLedger(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		tmux TmuxController
		want string
	}{
		{"session gone", sessionGoneFake{&fakeTmux{}}, "pane_lost"},
		{"session alive", &fakeTmux{}, "repl_boot_timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t, "claude-tmux", "")
			eng := newTestEngine(Deps{Tmux: tc.tmux, Sleep: func(time.Duration) {}, BootTimeoutS: 3})

			resp, _ := eng.Launch(context.Background(), core.BridgeRequest{
				CLI: "claude-tmux", Profile: fx.profile, Model: "auto",
				Prompt: "do the thing", Workspace: fx.ws, ArtifactPath: fx.artifact, Agent: "build",
			})

			if resp.ExitCode != ExitREPLBootTimeout || resp.CauseCode != tc.want {
				t.Fatalf("resp exit=%d cause=%q, want 80 with %q", resp.ExitCode, resp.CauseCode, tc.want)
			}
			rows, err := llmcalls.ReadWorkspace(fx.ws)
			if err != nil || len(rows.Records) == 0 || rows.Records[len(rows.Records)-1].CauseCode != tc.want {
				t.Fatalf("ledger rows=%+v err=%v, want the last row's cause %q", rows, err, tc.want)
			}
		})
	}
}

func TestRunScoped_ACallersObserversStillHearTheLaunch(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	var dispatched, lost bool
	deps := Deps{Tmux: sessionGoneFake{&fakeTmux{}}, Sleep: func(time.Duration) {}, BootTimeoutS: 3}
	deps.onModelDispatch = func(modelDispatch) { dispatched = true }
	deps.onBootLoss = func(launchoutcome.BootLossCause) { lost = true }

	newTestEngine(deps).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto",
		Prompt: "do the thing", Workspace: fx.ws, ArtifactPath: fx.artifact, Agent: "build",
	})

	if !dispatched || !lost {
		t.Fatalf("caller observers: dispatch=%v boot-loss=%v, want both chained", dispatched, lost)
	}

	fx = newFixture(t, "claude-tmux", "")
	artifact := filepath.Join(fx.ws, "triage-report.md")
	fatal := false
	scripted, _, _ := recordingDeps(t)
	scripted.Tmux = &scriptedSessionTmux{fakeTmux: &fakeTmux{}, dead: 1, artifact: artifact}
	scripted.Sleep = func(time.Duration) {}
	scripted.Reviewer = &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewExtend, Reason: "first observation has not crossed the persistence gate"}}}
	scripted.ArtifactTimeoutS = 2
	scripted.FatalPaneStage = "enforce"
	scripted.LookupEnv = mapLookup(nil)
	scripted.onFatalPane = func(fatalPaneObservation) { fatal = true }

	newTestEngine(scripted).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: "x",
		Workspace: fx.ws, ArtifactPath: artifact, Agent: "triage", Cycle: 1687, RunID: "run-1687",
	})

	if !fatal {
		t.Fatal("the caller's fatal-pane observer did not hear the dead shell")
	}
}
