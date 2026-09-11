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
