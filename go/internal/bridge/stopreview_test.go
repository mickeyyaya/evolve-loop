package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDeterministicReviewer covers the Stage-0 review decision: extend while
// the agent produces output (up to maxExtends), else pause. The key property —
// a progressing agent is never told to stop until the backstop — is what keeps
// a slow-but-working phase alive.
func TestDeterministicReviewer(t *testing.T) {
	r := newDeterministicReviewer(2)
	cases := []struct {
		name string
		ev   StopEvent
		want ReviewAction
	}{
		{"progressing, first interval → extend", StopEvent{Progressed: true, Attempt: 0}, ReviewExtend},
		{"progressing, under cap → extend", StopEvent{Progressed: true, Attempt: 1}, ReviewExtend},
		{"progressing, at cap → pause", StopEvent{Progressed: true, Attempt: 2}, ReviewPause},
		{"progressing, past cap → pause", StopEvent{Progressed: true, Attempt: 9}, ReviewPause},
		{"no output → pause immediately", StopEvent{Progressed: false, Attempt: 0}, ReviewPause},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Review(c.ev).Action; got != c.want {
				t.Fatalf("Review(%+v).Action = %q, want %q", c.ev, got, c.want)
			}
		})
	}
}

// TestDeterministicReviewer_NonPositiveMaxFallsBack guards that a 0/negative cap
// does not collapse to "pause at interval 0" (which would resurrect the
// kill-a-working-agent bug): it falls back to the default backstop.
func TestDeterministicReviewer_NonPositiveMaxFallsBack(t *testing.T) {
	for _, max := range []int{0, -1} {
		r := newDeterministicReviewer(max)
		if got := r.Review(StopEvent{Progressed: true, Attempt: 0}).Action; got != ReviewExtend {
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

// scriptedReviewer returns a fixed sequence of verdicts, recording the events
// it saw. Once the script is exhausted it pauses (so a test can never spin
// forever waiting on an artifact that never lands).
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

// TestRunTmuxREPL_ContextCancelledBreaks proves the wait loop honours context
// cancellation even under a reviewer that would extend forever — so an
// orchestrator timeout / SIGTERM is not swallowed by the extend budget.
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

// runTmuxRev mirrors runTmux but injects a custom StopReviewer so a test can
// drive the artifact-wait review loop deterministically.
func runTmuxRev(t *testing.T, fx launchFixture, tmux *fakeTmux, rev StopReviewer, lookup map[string]string, extra ...string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(lookup),
		Reviewer:  rev,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

// TestRunTmuxREPL_ReviewExtendThenPause proves the wait loop honours the
// reviewer: two extensions keep it waiting past the first interval (the old
// wall-clock would have killed it), then a pause verdict ends it as a timeout.
func TestRunTmuxREPL_ReviewExtendThenPause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "working"},
		{Action: ReviewExtend, Reason: "working"},
		{Action: ReviewPause, Reason: "stalled"},
	}}
	// Tiny interval so each loop iteration crosses a review boundary.
	code, stderr := runTmuxRev(t, fx, tmux, rev, map[string]string{"EVOLVE_ARTIFACT_TIMEOUT_S": "2"}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout after pause); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if len(rev.events) != 3 {
		t.Fatalf("reviewer called %d times, want 3 (extend, extend, pause)", len(rev.events))
	}
	// Attempt counter advances only on extension.
	for i, want := range []int{0, 1, 2} {
		if rev.events[i].Attempt != want {
			t.Fatalf("event[%d].Attempt = %d, want %d", i, rev.events[i].Attempt, want)
		}
	}
	if rev.events[0].Kind != StopArtifactTimeout {
		t.Fatalf("event kind = %q, want %q", rev.events[0].Kind, StopArtifactTimeout)
	}
}

// TestRunTmuxREPL_ArtifactAppears_NoReview proves the fast path is unchanged:
// when the artifact is already present the loop exits on the first poll and the
// reviewer is never consulted.
func TestRunTmuxREPL_ArtifactAppears_NoReview(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{}
	code, stderr := runTmuxRev(t, fx, tmux, rev, nil, "--allow-bypass")

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if len(rev.events) != 0 {
		t.Fatalf("reviewer called %d times, want 0 (artifact present → no review)", len(rev.events))
	}
}
