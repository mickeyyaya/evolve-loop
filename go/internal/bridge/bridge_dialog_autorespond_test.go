package bridge

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Deprecated: TestAgyTmuxManifest_SessionRating_AutoResponds
func TestAgyTmuxManifest_SessionRating_AutoResponds(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	panes := []struct {
		name string
		pane string
	}{
		{name: "rate_this_response", pane: "│ Rate this response\n│ 1  2  3  4  5\n? for shortcuts"},
		{name: "how_helpful_session", pane: "How helpful was this session?\n(press a number)\n? for shortcuts"},
	}
	for _, tc := range panes {
		t.Run(tc.name, func(t *testing.T) {
			action, rc := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, false)
			if rc != 1 || !strings.HasPrefix(action, "send:") {
				t.Errorf("rating dialog must auto-respond; got action=%q rc=%d (want send:* rc=1)", action, rc)
			}
			if keys := strings.TrimPrefix(action, "send:"); strings.TrimSpace(keys) == "" {
				t.Errorf("auto-response must carry a non-empty key sequence; got %q", action)
			}
		})
	}
}

func TestAgyTmuxManifest_SessionRating_NoFalsePositives(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	t.Run("normal_output_noops", func(t *testing.T) {
		pane := "✦ Writing scout-report.md to the workspace…\ndone.\n? for shortcuts"
		action, rc := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false)
		if rc != 0 {
			t.Errorf("ordinary working pane must not trigger any rule; got action=%q rc=%d", action, rc)
		}
	})
	t.Run("auth_prompt_still_escalates", func(t *testing.T) {
		action, rc := decideAutoRespond("Please log in to continue", m.InteractivePrompts, map[string]int{}, false)
		if rc != 85 {
			t.Errorf("auth pane must still escalate; got action=%q rc=%d", action, rc)
		}
	})
}

// nudgeRecordingTmux also records pasted buffer content, so a nudge counts whether the driver pastes it or sends keys.
type nudgeRecordingTmux struct {
	*fakeTmux
	pastes []string
}

func (n *nudgeRecordingTmux) LoadBuffer(ctx context.Context, session, file string) error {
	if b, err := os.ReadFile(file); err == nil {
		n.pastes = append(n.pastes, string(b))
	}
	return n.fakeTmux.LoadBuffer(ctx, session, file)
}

// deliveriesNaming counts pasted buffers and sent keys that mention sub; the fixture prompt and launch line never hold the artifact path.
func (n *nudgeRecordingTmux) deliveriesNaming(sub string) int {
	count := 0
	for _, p := range n.pastes {
		if strings.Contains(p, sub) {
			count++
		}
	}
	for _, k := range n.sentKeys {
		if strings.Contains(k, sub) {
			count++
		}
	}
	return count
}

// runTmuxNudge drives an idle claude-tmux launch under a 30s deadline, so a runaway nudge loop fails the count instead of hanging go test.
func runTmuxNudge(t *testing.T, fx launchFixture, tmux *nudgeRecordingTmux) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:            tmux,
		Sleep:           func(time.Duration) {},
		LookupEnv:       mapLookup(nil),
		CaptureBaseline: zeroBaselineCapture})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	return code, stderr.String()
}

func TestTmuxREPL_IdleArtifactNudge_SentOnceThenTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if got := tmux.deliveriesNaming(fx.artifact); got != 1 {
		t.Errorf("idle-artifact nudge naming %s must be delivered exactly once; got %d deliveries", fx.artifact, got)
	}
}

func TestTmuxREPL_NoNudgeWhenArtifactPresent(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if got := tmux.deliveriesNaming(fx.artifact); got != 0 {
		t.Errorf("no nudge may be delivered when the artifact is present; got %d", got)
	}
}
