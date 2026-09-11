package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

type waitOrderTmux struct {
	fakeTmux
	pane                         string
	observedFallback             string
	completionCaptures           int
	completionCaptureSawFallback bool
}

func (t *waitOrderTmux) setPane(pane string) {
	t.pane = pane
}

func (t *waitOrderTmux) CapturePane(_ context.Context, _ string, scrollback int) (string, error) {
	t.captureScrollback = append(t.captureScrollback, scrollback)
	t.lastPane = t.pane
	if scrollback == 0 && strings.Contains(t.pane, "completion-tick") {
		t.completionCaptures++
		t.completionCaptureSawFallback = regularFileNonEmpty(t.observedFallback)
	}
	return t.pane, nil
}

func TestRunTmuxREPL_CompletionDetectorErrorIsReportedOnce(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	blockedParent := filepath.Join(fx.ws, "blocked")
	fx.artifact = filepath.Join(blockedParent, "artifact.md")
	fallback := filepath.Join(fx.ws, "workspace", filepath.Base(fx.artifact))
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "stop after repeated detector failures"}}}

	waitTicks := 0
	sleep := func(delay time.Duration) {
		if delay != 2*time.Second {
			return
		}
		waitTicks++
		if waitTicks != 1 {
			return
		}
		if err := os.RemoveAll(blockedParent); err != nil {
			t.Fatalf("replace canonical parent: %v", err)
		}
		if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("block canonical parent: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
			t.Fatalf("create fallback parent: %v", err)
		}
		if err := os.WriteFile(fallback, []byte("deliverable at fallback"), 0o644); err != nil {
			t.Fatalf("write fallback artifact: %v", err)
		}
	}

	eng := newTestEngine(Deps{
		Tmux:               tmux,
		Sleep:              sleep,
		Reviewer:           reviewer,
		ArtifactTimeoutS:   8,
		ArtifactMaxExtends: 1,
		CaptureBaseline:    zeroBaselineCapture,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr:\n%s", code, stderr.String())
	}
	if waitTicks < 4 {
		t.Fatalf("wait ticks = %d, want repeated detector polls before review", waitTicks)
	}
	const warning = "WARN: completion detector:"
	if got := strings.Count(stderr.String(), warning); got != 1 {
		t.Fatalf("completion-detector warnings = %d, want exactly 1; stderr:\n%s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a directory") {
		t.Fatalf("detector warning omitted relocation cause; stderr:\n%s", stderr.String())
	}
	summary := artifactTimeoutSummary(stderr.String())
	if !strings.Contains(summary, "cause=completion_detector_error") {
		t.Errorf("terminal detector failure lost its cause code; summary=%q", summary)
	}
	if !strings.Contains(summary, `detector_error="`) || !strings.Contains(summary, "not a directory") {
		t.Errorf("terminal detector failure lost its concrete error; summary=%q", summary)
	}
}

func TestRunTmuxREPL_DetectorErrorThenIncompleteStopRetainsSecondaryEvidence(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	blockedParent := filepath.Join(fx.ws, "blocked")
	fx.artifact = filepath.Join(blockedParent, "artifact.md")
	fallback := filepath.Join(fx.ws, "workspace", filepath.Base(fx.artifact))
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewStop, Reason: "reviewer stopped an incomplete wait"}}}

	waitTicks := 0
	sleep := func(delay time.Duration) {
		if delay != 2*time.Second {
			return
		}
		waitTicks++
		switch waitTicks {
		case 1:
			if err := os.RemoveAll(blockedParent); err != nil {
				t.Fatalf("replace canonical parent: %v", err)
			}
			if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("block canonical parent: %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
				t.Fatalf("create fallback parent: %v", err)
			}
			if err := os.WriteFile(fallback, []byte("deliverable at fallback"), 0o644); err != nil {
				t.Fatalf("write fallback artifact: %v", err)
			}
		case 3:
			if err := os.Remove(fallback); err != nil {
				t.Fatalf("remove fallback after detector error: %v", err)
			}
		}
	}

	eng := newTestEngine(Deps{
		Tmux:               tmux,
		Sleep:              sleep,
		Reviewer:           reviewer,
		ArtifactTimeoutS:   8,
		ArtifactMaxExtends: 1,
		CaptureBaseline:    zeroBaselineCapture,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr:\n%s", code, stderr.String())
	}
	summary := artifactTimeoutSummary(stderr.String())
	if !strings.Contains(summary, "cause=review_stop") {
		t.Errorf("error-free terminal observation did not yield to reviewer stop; summary=%q", summary)
	}
	if !strings.Contains(summary, `detector_error="`) || !strings.Contains(summary, "not a directory") {
		t.Errorf("prior detector failure was not retained as secondary evidence; summary=%q", summary)
	}
}

func TestRunTmuxREPL_DetectorErrorThenConfirmedCompletionHasNoTimeoutDiagnostic(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	blockedParent := filepath.Join(fx.ws, "blocked")
	fx.artifact = filepath.Join(blockedParent, "artifact.md")
	fallback := filepath.Join(fx.ws, "workspace", filepath.Base(fx.artifact))
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewStop, Reason: "must not run"}}}

	waitTicks := 0
	sleep := func(delay time.Duration) {
		if delay != 2*time.Second {
			return
		}
		waitTicks++
		switch waitTicks {
		case 1:
			if err := os.RemoveAll(blockedParent); err != nil {
				t.Fatalf("replace canonical parent: %v", err)
			}
			if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("block canonical parent: %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
				t.Fatalf("create fallback parent: %v", err)
			}
			if err := os.WriteFile(fallback, []byte("deliverable at fallback"), 0o644); err != nil {
				t.Fatalf("write fallback artifact: %v", err)
			}
		case 3:
			if err := os.Remove(fallback); err != nil {
				t.Fatalf("remove fallback after detector error: %v", err)
			}
			if err := os.Remove(blockedParent); err != nil {
				t.Fatalf("remove canonical blocker: %v", err)
			}
			if err := os.MkdirAll(blockedParent, 0o755); err != nil {
				t.Fatalf("create canonical parent: %v", err)
			}
			body := "<!-- challenge-token: " + fx.token + " -->\nDONE\n"
			if err := os.WriteFile(fx.artifact, []byte(body), 0o644); err != nil {
				t.Fatalf("write canonical artifact: %v", err)
			}
		}
	}

	eng := newTestEngine(Deps{
		Tmux:               tmux,
		Sleep:              sleep,
		Reviewer:           reviewer,
		ArtifactTimeoutS:   20,
		ArtifactMaxExtends: 1,
		CaptureBaseline:    zeroBaselineCapture,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK after confirmed completion; stderr:\n%s", code, stderr.String())
	}
	if len(reviewer.events) != 0 {
		t.Fatalf("reviewer ran after completion: %+v", reviewer.events)
	}
	if strings.Contains(stderr.String(), artifactTimeoutMarker) {
		t.Fatalf("successful completion emitted a timeout diagnostic; stderr=%q", stderr.String())
	}
}

func TestRunTmuxREPL_OrdinaryCompletionPrecedesTickEffects(t *testing.T) {
	for _, tc := range []struct {
		name         string
		stage        string
		wantCaptures int
	}{
		{name: "channel_off_polls_before_capture", stage: "off", wantCaptures: 0},
		{name: "channel_on_captures_before_poll", stage: "enforce", wantCaptures: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t, "claude-tmux", "")
			fallback := filepath.Join(fx.ws, "workspace", filepath.Base(fx.artifact))
			tmux := &waitOrderTmux{pane: tmuxPromptMarkerDefault, observedFallback: fallback}
			reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "must not run"}}}
			wallProbes := 0
			waitTick := 0
			sleep := func(delay time.Duration) {
				if delay != 2*time.Second {
					return
				}
				waitTick++
				switch waitTick {
				case 1:
					tmux.setPane("❯ first-wall-tick\nYou've reached your Fable 5 limit.\n❯")
					if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
						t.Fatalf("create fallback parent: %v", err)
					}
					if err := os.WriteFile(fallback, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
						t.Fatalf("write completing fallback artifact: %v", err)
					}
				case 2:
					tmux.setPane("❯ completion-tick\nYou've reached your Fable 5 limit.\n❯")
					if err := inbox.Append(fx.ws, "build", inbox.Envelope{
						Kind: inbox.KindKeystroke, Body: "F13", Source: "test",
					}, fixedTime); err != nil {
						t.Fatalf("append competing inbox effect: %v", err)
					}
				}
			}

			eng := newTestEngine(Deps{
				Tmux:               tmux,
				Sleep:              sleep,
				Reviewer:           reviewer,
				ArtifactTimeoutS:   2,
				ArtifactMaxExtends: 1,
				RecoveryStage:      tc.stage,
				CorroborateWall: func(context.Context, string) bool {
					wallProbes++
					return false
				},
			})
			var stdout, stderr bytes.Buffer
			code := eng.LaunchArgs(context.Background(),
				fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)

			if code != ExitOK {
				t.Fatalf("exit = %d, want ExitOK; stderr:\n%s", code, stderr.String())
			}
			if tmux.completionCaptures != tc.wantCaptures {
				t.Fatalf("completion-tick captures = %d, want %d", tmux.completionCaptures, tc.wantCaptures)
			}
			if tc.wantCaptures > 0 && !tmux.completionCaptureSawFallback {
				t.Fatal("channel-on completion capture did not observe the fallback before detector relocation")
			}
			if tmux.sentContains("F13") {
				t.Fatalf("completion tick drained a competing inbox envelope; sent=%v", tmux.sentSeq)
			}
			if wallProbes != 0 {
				t.Fatalf("completion tick ran %d wall probe(s), want 0", wallProbes)
			}
			if len(reviewer.events) != 0 {
				t.Fatalf("completion tick invoked reviewer with %+v", reviewer.events)
			}
		})
	}
}

func TestRunTmuxREPL_InboxPrecedesAutoRespondOnSameTick(t *testing.T) {
	swapManifestFS(t, fakeManifestFS{files: map[string][]byte{
		"manifests/tick-order.json": []byte(`{"cli":"tick-order","binary":"x","interactive_prompts":[{"name":"waiting","regex":"WAITING","response_keys":"AUTO","policy":"auto_respond"}]}`),
	}})

	ws := t.TempDir()
	promptFile := writeJSON(t, filepath.Join(ws, "prompt.txt"), "do the work")
	cfg := &Config{
		Model:       "m",
		PromptFile:  promptFile,
		ProjectRoot: ws,
		Workspace:   ws,
		Worktree:    ws,
		Agent:       "build",
		Artifact:    filepath.Join(ws, "artifact.md"),
		StdoutLog:   filepath.Join(ws, "stdout.log"),
		StderrLog:   filepath.Join(ws, "stderr.log"),
	}
	deps := covDeps()
	tmux := &fakeTmux{paneSeq: []string{"❯ WAITING"}}
	deps.Tmux = tmux

	waitTicks := 0
	deps.Sleep = func(delay time.Duration) {
		if delay != 2*time.Second {
			return
		}
		waitTicks++
		switch waitTicks {
		case 1:
			if err := inbox.Append(ws, "build", inbox.Envelope{
				Kind: inbox.KindKeystroke, Body: "F13", Source: "test",
			}, fixedTime); err != nil {
				t.Fatalf("append inbox effect: %v", err)
			}
		case 2:
			if err := os.WriteFile(cfg.Artifact, []byte("done"), 0o644); err != nil {
				t.Fatalf("write completing artifact: %v", err)
			}
		}
	}

	lp := tmuxLaunch{
		name: "tick-order", session: "s", launchCmd: "x",
		promptMarker: "❯", bootIntervalS: 1,
	}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}

	inboxAt := indexOf(tmux.sentSeq, "F13|false")
	autoAt := indexOf(tmux.sentSeq, "AUTO|false")
	if inboxAt < 0 || autoAt < 0 {
		t.Fatalf("missing same-tick effects: inbox=%d auto=%d sent=%v", inboxAt, autoAt, tmux.sentSeq)
	}
	if inboxAt >= autoAt {
		t.Fatalf("inbox effect must precede auto-response on the same tick: inbox=%d auto=%d sent=%v", inboxAt, autoAt, tmux.sentSeq)
	}
}

func TestRunTmuxREPL_CancelledCompletionPrecedesTickEffects(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &waitOrderTmux{pane: tmuxPromptMarkerDefault}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "must not run"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waitTick := 0
	wallProbes := 0
	sleep := func(delay time.Duration) {
		if delay != 2*time.Second {
			return
		}
		waitTick++
		if waitTick != 1 {
			return
		}
		tmux.setPane("❯ completion-tick\nYou've reached your Fable 5 limit.\n❯")
		if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
			t.Fatalf("write final-poll artifact: %v", err)
		}
		if err := inbox.Append(fx.ws, "build", inbox.Envelope{
			Kind: inbox.KindKeystroke, Body: "F14", Source: "test",
		}, fixedTime); err != nil {
			t.Fatalf("append competing inbox effect: %v", err)
		}
		cancel()
	}

	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            sleep,
		Reviewer:         reviewer,
		ArtifactTimeoutS: 2,
		RecoveryStage:    "enforce",
		CorroborateWall: func(context.Context, string) bool {
			wallProbes++
			return true
		},
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx,
		fx.args("claude-tmux", "--allow-bypass", "--agent=build"), nil, &stdout, &stderr)

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK from the detached final poll; stderr:\n%s", code, stderr.String())
	}
	if tmux.completionCaptures != 0 {
		t.Fatalf("cancelled completion captured the ordinary tick pane %d time(s), want 0", tmux.completionCaptures)
	}
	if tmux.sentContains("F14") || wallProbes != 0 || len(reviewer.events) != 0 {
		t.Fatalf("cancelled completion ran later tick effects: sent=%v wall_probes=%d reviews=%+v",
			tmux.sentSeq, wallProbes, reviewer.events)
	}
}
