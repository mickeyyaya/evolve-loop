package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// permanentlyParkedPromptTmux models a prompt paste whose input line remains
// visible despite every bounded re-send of Enter.
type permanentlyParkedPromptTmux struct {
	*fakeTmux
	pasted bool
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
		return unsubmittedPane("write the artifact"), err
	}
	return pane, err
}

// TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget reproduces the
// cycle-1510 failure: submit verification detects a wedged prompt, but the
// result is discarded and the driver starts the normal artifact-wait loop.
func TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget(t *testing.T) {
	cfg := fixtureConfig(t)
	tm := &permanentlyParkedPromptTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
	}
	deps := fixtureDeps(tm)
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	artifactWaitPolls := 0
	deps.Sleep = func(d time.Duration) {
		if d == 2*time.Second {
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
	if !strings.Contains(stderr.String(), artifactTimeoutMarker+"phase=build") ||
		!strings.Contains(stderr.String(), "reason=\"prompt submit_wedged (resends=3)\"") {
		t.Fatalf("early timeout must preserve the classified submit_wedged cause in the artifact-timeout marker; stderr=%s", stderr.String())
	}
}
