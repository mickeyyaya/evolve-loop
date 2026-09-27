package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// jiggleTmux upgrades fakeTmux with the optional windowJiggler capability.
type jiggleTmux struct {
	fakeTmux
	jiggles int
}

func (j *jiggleTmux) JiggleWindow(_ context.Context, _ string) error {
	j.jiggles++
	return nil
}

func runTmuxWedge(t *testing.T, fx launchFixture, tmux TmuxController, spy func(phase, action, reason string)) (int, string) {
	t.Helper()
	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		ArtifactTimeoutS: 2,
		OnStopReview:     spy,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	return code, stderr.String()
}

// TestRunTmuxREPL_BlankPaneWedge_JigglesAndExtends: pane boots, renders once,
// then goes permanently blank while the session stays alive. The driver must
// jiggle and extend (busy) every interval until the maxExtends backstop
// exhausts.
func TestRunTmuxREPL_BlankPaneWedge_JigglesAndExtends(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault, // boot
		"⏺ working on it…",      // post-paste baseline
		"",                      // every subsequent capture: blank (repeats)
	}}}
	var got []stopReviewRec
	code, stderr := runTmuxWedge(t, fx, tmux, func(phase, action, reason string) {
		got = append(got, stopReviewRec{phase, action, reason})
	})

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (artifact never lands); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if tmux.jiggles == 0 {
		t.Error("blank pane with live session: driver never jiggled the window (no SIGWINCH redraw attempt)")
	}
	if len(got) == 0 {
		t.Fatal("no stop-review verdicts recorded")
	}
	for i, r := range got {
		if strings.Contains(r.reason, "stalled; pause for investigation") {
			t.Errorf("verdict %d is the legacy blank-pane stall-pause (%q) — a live wedged pane must extend", i, r.reason)
		}
	}
	if got[0].action != string(ReviewExtend) {
		t.Errorf("first verdict = %s (%s), want extend (render wedge reads busy)", got[0].action, got[0].reason)
	}
	// The backstop still bounds a genuinely-hung agent: the LAST verdict is
	// the exhausted pause, everything before extends.
	last := got[len(got)-1]
	if last.action != string(ReviewPause) || !strings.Contains(last.reason, "exhausted") {
		t.Errorf("final verdict = %s (%s), want the exhausted-extensions pause backstop", last.action, last.reason)
	}
}

// TestRecoverBlankPane_Unit pins the recovery helper deterministically (the
// driver-level FIFO frames can't align a one-shot recovery: the autorespond
// tick also captures every iteration). Cases: jiggle recovers content /
// wedge persists / non-blank no-op / dead session no-op.
func TestRecoverBlankPane_Unit(t *testing.T) {
	mk := func(frames []string, alive bool) (*jiggleTmux, Deps, *bytes.Buffer) {
		tm := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: frames}}
		if alive {
			tm.existing = map[string]bool{"s": true}
		}
		var errBuf bytes.Buffer
		return tm, Deps{Tmux: tm, Sleep: func(time.Duration) {}, Stderr: &errBuf}, &errBuf
	}

	t.Run("jiggle recovers content", func(t *testing.T) {
		tm, deps, errBuf := mk([]string{"✻ Kneading… (6s · ↓ 244 tokens)"}, true)
		pane, wedged := recoverBlankPane(context.Background(), deps, "s", 100, "  \n ", "[t]")
		if wedged || !strings.Contains(pane, "Kneading") {
			t.Errorf("recovered pane = %q wedged=%v, want spinner frame, not wedged", pane, wedged)
		}
		if tm.jiggles != 1 {
			t.Errorf("jiggles = %d, want 1", tm.jiggles)
		}
		if !strings.Contains(errBuf.String(), "redrawn after jiggle") {
			t.Errorf("recovery breadcrumb missing: %s", errBuf.String())
		}
	})
	t.Run("wedge persists", func(t *testing.T) {
		tm, deps, errBuf := mk([]string{""}, true)
		_, wedged := recoverBlankPane(context.Background(), deps, "s", 100, "", "[t]")
		if !wedged {
			t.Error("still-blank pane must report wedged")
		}
		if tm.jiggles != 1 {
			t.Errorf("jiggles = %d, want 1", tm.jiggles)
		}
		if !strings.Contains(errBuf.String(), "treating live session as busy") {
			t.Errorf("wedge breadcrumb missing: %s", errBuf.String())
		}
	})
	t.Run("non-blank pane is a no-op", func(t *testing.T) {
		tm, deps, _ := mk([]string{"unused"}, true)
		pane, wedged := recoverBlankPane(context.Background(), deps, "s", 100, "content", "[t]")
		if wedged || pane != "content" || tm.jiggles != 0 {
			t.Errorf("no-op violated: pane=%q wedged=%v jiggles=%d", pane, wedged, tm.jiggles)
		}
	})
	t.Run("dead session is not a wedge", func(t *testing.T) {
		tm, deps, _ := mk([]string{"unused"}, false)
		_, wedged := recoverBlankPane(context.Background(), deps, "s", 100, "", "[t]")
		if wedged || tm.jiggles != 0 {
			t.Errorf("dead session must skip recovery: wedged=%v jiggles=%d", wedged, tm.jiggles)
		}
	})
}

// TestRunTmuxREPL_BlankAtBaseline_NotWedge — an idle prompt pane (marker
// rendered, not blank) must NOT trigger jiggles: the wedge path is strictly
// blank-while-alive.
func TestRunTmuxREPL_BlankAtBaseline_NotWedge(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault, // boot
		tmuxPromptMarkerDefault, // idle prompt forever (repeats)
	}}}
	_, _ = runTmuxWedge(t, fx, tmux, nil)
	if tmux.jiggles != 0 {
		t.Errorf("idle (non-blank) pane must not be jiggled; got %d jiggles", tmux.jiggles)
	}
}
