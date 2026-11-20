package bridge

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type permanentlyParkedPromptTmux struct {
	*fakeTmux
	pasted     bool
	parkedText string
}

func (p *permanentlyParkedPromptTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := p.fakeTmux.PasteBuffer(ctx, session); err != nil {
		return err
	}
	p.pasted = true
	return nil
}

func (p *permanentlyParkedPromptTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	pane, err := p.fakeTmux.CapturePane(ctx, session, scrollback)
	if p.pasted {
		text := p.parkedText
		if text == "" {
			text = "write the artifact"
		}
		return unsubmittedPane(text), err
	}
	return pane, err
}

func TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget(t *testing.T) {
	t.Parallel()
	cfg := fixtureConfig(t)
	tm := &permanentlyParkedPromptTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
	}
	deps := fixtureDeps(tm)
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	artifactWaitPolls := 0
	deps.Sleep = func(d time.Duration) {
		if d == artifactWaitInterval && strings.Contains(stderr.String(), "STILL unsubmitted") {
			artifactWaitPolls++
		}
	}

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name:            "claude-tmux",
		session:         "submit-wedged",
		launchCmd:       "claude",
		promptMarker:    tmuxPromptMarkerDefault,
		inputLineMarker: tmuxPromptMarkerDefault,
		bootIntervalS:   1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d, %v), want (%d, nil); stderr=%s", code, err, ExitArtifactTimeout, stderr.String())
	}
	if artifactWaitPolls != 0 {
		t.Fatalf("submit_wedged entered %d normal artifact-wait poll(s); want immediate timeout before consuming the silence budget; stderr=%s", artifactWaitPolls, stderr.String())
	}
	if !strings.Contains(stderr.String(), artifactTimeoutMarker+"cause=submit_wedged") ||
		!strings.Contains(stderr.String(), "phase=build") ||
		!strings.Contains(stderr.String(), fmt.Sprintf("reason=\"prompt submit_wedged (resends=%d)\"", len(defaultSubmitResendSchedule))) {
		t.Fatalf("early timeout must preserve the classified submit_wedged cause in the artifact-timeout marker; stderr=%s", stderr.String())
	}
}

func TestEngineLaunch_SubmitResendScheduleReachesVerifySubmitted(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tm := &permanentlyParkedPromptTmux{
		fakeTmux:   &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		parkedText: "[Pasted text #1 +10 lines]",
	}
	var stderr bytes.Buffer
	var sleeps []time.Duration
	eng := NewEngine(Deps{
		Tmux:                 tm,
		Stderr:               &stderr,
		LookupEnv:            mapLookup(nil),
		SubmitResendSchedule: []time.Duration{3 * time.Second, 7 * time.Second},
		Sleep: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
	})
	var stdout bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=2550"), nil, &stdout, &stderr)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `reason="prompt submit_wedged (resends=2)"`) {
		t.Fatalf("stderr missing custom 2-resend wedge reason; stderr=%s", stderr.String())
	}
	var got []time.Duration
	for _, d := range sleeps {
		if d == 3*time.Second || d == 3*time.Second+500*time.Millisecond {
			got = append(got, d)
		}
	}
	want := []time.Duration{3 * time.Second, 3*time.Second + 500*time.Millisecond}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("custom schedule intervals = %v (all sleeps %v), want %v", got, sleeps, want)
	}
}
