package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func TestDeterministicReviewer(t *testing.T) {
	r := newDeterministicReviewer(2)
	cases := []struct {
		name string
		ev   StopEvent
		want ReviewAction
	}{
		{"progressing, first interval → extend", StopEvent{State: panestream.LivenessConverging, Attempt: 0}, ReviewExtend},
		{"progressing, under cap → extend", StopEvent{State: panestream.LivenessConverging, Attempt: 1}, ReviewExtend},
		{"progressing, at cap → EXTEND (real output is never stuck)", StopEvent{State: panestream.LivenessConverging, Attempt: 2}, ReviewExtend},
		{"progressing, past cap → EXTEND (cycle-311/312: a producing scout killed mid-work)", StopEvent{State: panestream.LivenessConverging, Attempt: 9}, ReviewExtend},
		{"no output → pause immediately", StopEvent{State: panestream.LivenessIdle, Attempt: 0}, ReviewPause},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Review(c.ev).Action; got != c.want {
				t.Fatalf("Review(%+v).Action = %q, want %q", c.ev, got, c.want)
			}
		})
	}
}

func TestDeterministicReviewer_BusyPaneIsLiveness(t *testing.T) {
	r := newDeterministicReviewer(2)
	cases := []struct {
		name string
		ev   StopEvent
		want ReviewAction
	}{
		{"busy AND progressing → extend", StopEvent{State: panestream.LivenessConverging, Progressed: true, Busy: true, Attempt: 0}, ReviewExtend},
		{"busy, no delta, first interval → extend", StopEvent{State: panestream.LivenessBusyButStagnant, Busy: true, Attempt: 0}, ReviewExtend},
		{"busy, no delta, under cap → extend", StopEvent{State: panestream.LivenessBusyButStagnant, Busy: true, Attempt: 1}, ReviewExtend},
		{"busy, no delta, at cap → pause (backstop)", StopEvent{State: panestream.LivenessBusyButStagnant, Busy: true, Attempt: 2}, ReviewPause},
		{"idle, no delta → pause immediately", StopEvent{State: panestream.LivenessIdle, Attempt: 0}, ReviewPause},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Review(c.ev).Action; got != c.want {
				t.Fatalf("Review(%+v).Action = %q, want %q", c.ev, got, c.want)
			}
		})
	}
}

func TestDeterministicReviewer_NonPositiveMaxFallsBack(t *testing.T) {
	for _, max := range []int{0, -1} {
		r := newDeterministicReviewer(max)
		if got := r.Review(StopEvent{State: panestream.LivenessConverging, Attempt: 0}).Action; got != ReviewExtend {
			t.Fatalf("newDeterministicReviewer(%d): first progressing interval = %q, want extend", max, got)
		}
	}
}

func TestEnvInt(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{"unset → default", false, "", 300},
		{"valid", true, "900", 900},
		{"empty → default", true, "", 300},
		{"non-numeric → default", true, "abc", 300},
		{"zero → default", true, "0", 300},
		{"negative → default", true, "-5", 300},
		{"whitespace trimmed", true, " 120 ", 120},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := map[string]string{}
			if c.set {
				m["K"] = c.val
			}
			if got := envInt(Deps{LookupEnv: mapLookup(m)}, "K", 300); got != c.want {
				t.Fatalf("envInt = %d, want %d", got, c.want)
			}
		})
	}
}

// scriptedReviewer pauses once its verdict script is exhausted, so a test can
// never spin forever waiting on an artifact that never lands.
type scriptedReviewer struct {
	verdicts []ReviewVerdict
	events   []StopEvent
}

func (s *scriptedReviewer) Review(ev StopEvent) ReviewVerdict {
	s.events = append(s.events, ev)
	if len(s.events) <= len(s.verdicts) {
		return s.verdicts[len(s.events)-1]
	}
	return ReviewVerdict{Action: ReviewPause, Reason: "script exhausted"}
}

// alwaysExtendReviewer never stops on its own — only an external signal
// (context cancellation) can end a wait it governs.
type alwaysExtendReviewer struct{}

func (alwaysExtendReviewer) Review(StopEvent) ReviewVerdict {
	return ReviewVerdict{Action: ReviewExtend, Reason: "always extend"}
}

func TestRunTmuxREPL_ContextCancelledBreaks(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"❯"}} // boots immediately, artifact never appears
	deps.Reviewer = alwaysExtendReviewer{}
	lp := tmuxLaunch{name: "claude", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled: the first wait iteration must break
	code, _ := runTmuxREPL(ctx, cfg, deps, lp)
	if code != ExitArtifactTimeout {
		t.Fatalf("cancelled context should break the wait → ExitArtifactTimeout; got %d", code)
	}
}

func runTmuxRev(t *testing.T, fx launchFixture, tmux *fakeTmux, rev StopReviewer, extraDeps Deps, extra ...string) (int, string) {
	t.Helper()
	d := extraDeps
	d.Tmux = tmux
	d.Sleep = func(time.Duration) {}
	if d.CaptureBaseline == nil {
		// Harness default ONLY: pre-seeded artifacts stand in for mid-session
		// writes. A stale-pre-dispatch scenario must inject the REAL capture
		// explicitly (see the baseline wiring test).
		d.CaptureBaseline = zeroBaselineCapture
	}
	d.Reviewer = rev
	eng := newTestEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

func TestRunTmuxREPL_ReviewExtendThenPause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "working"},
		{Action: ReviewExtend, Reason: "working"},
		{Action: ReviewPause, Reason: "stalled"},
	}}
	// Tiny interval so each loop iteration crosses a review boundary.
	code, stderr := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout after pause); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if len(rev.events) != 3 {
		t.Fatalf("reviewer called %d times, want 3 (extend, extend, pause)", len(rev.events))
	}
	for i, want := range []int{0, 1, 2} {
		if rev.events[i].Attempt != want {
			t.Fatalf("event[%d].Attempt = %d, want %d", i, rev.events[i].Attempt, want)
		}
	}
	if rev.events[0].Kind != StopArtifactTimeout {
		t.Fatalf("event kind = %q, want %q", rev.events[0].Kind, StopArtifactTimeout)
	}
}

func TestRunTmuxREPL_ArtifactAppears_NoReview(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{}
	code, stderr := runTmuxRev(t, fx, tmux, rev, Deps{}, "--allow-bypass")

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if len(rev.events) != 0 {
		t.Fatalf("reviewer called %d times, want 0 (artifact present → no review)", len(rev.events))
	}
}
