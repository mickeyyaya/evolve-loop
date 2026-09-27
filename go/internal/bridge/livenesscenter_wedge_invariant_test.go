package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func TestWedgeCorpus_Converging_ProducingNeverCapped(t *testing.T) {
	r := newDeterministicReviewer(2)
	ev := StopEvent{State: panestream.LivenessConverging, Attempt: 50}
	if got := r.Review(ev).Action; got != ReviewExtend {
		t.Fatalf("Converging at attempt 50 (maxExtends=2) = %q, want extend (cycle-311/312: producing agent never capped)", got)
	}
}

func TestWedgeCorpus_BusyStagnant_BoundedThenPause(t *testing.T) {
	r := newDeterministicReviewer(2)
	if got := r.Review(StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 1}).Action; got != ReviewExtend {
		t.Fatalf("BusyButStagnant under cap (attempt 1/2) = %q, want extend", got)
	}
	if got := r.Review(StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 2}).Action; got != ReviewPause {
		t.Fatalf("BusyButStagnant at cap (attempt 2/2) = %q, want pause (cycle-254/255 bound)", got)
	}
}

func TestWedgeCorpus_DeadPane_HungIsNotConverging(t *testing.T) {
	r := newDeterministicReviewer(6)
	got := r.Review(StopEvent{State: panestream.LivenessHung, Attempt: 0}).Action
	if got != ReviewPause {
		t.Fatalf("Hung at attempt 0 = %q, want pause (cycle-262: a dead/echoing pane must fast-fail, not read as Converging)", got)
	}
}

func TestWedgeCorpus_EvidenceSurvivesEmptyCapture(t *testing.T) {
	cfg := fixtureConfig(t)
	const evidence = "TOOL CALL: go test ./... — last real output before server death"
	base := &FakeTmuxController{CaptureFrames: []string{"❯", evidence}}
	tm := &dyingServerTmux{FakeTmuxController: base, aliveCaps: 2}
	rev := &scriptedReviewer{}
	deps := fixtureDeps(tm)
	deps.Reviewer = rev

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "wedge-evidence", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d,%v), want (ExitArtifactTimeout,nil)", code, err)
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	for i, ev := range rev.events {
		if !strings.Contains(ev.StdoutTail, evidence) {
			t.Errorf("checkpoint %d StdoutTail lost the cycle-286/288 evidence after server death; got %q", i, ev.StdoutTail)
		}
	}
}

// Anti-gaming: the corpus above must exercise the production reviewer type, not a test double standing in
// for it — a stubbed StopReviewer could satisfy any verdict table without the real Review() logic running.
func TestWedgeCorpus_UsesRealDeterministicReviewer(t *testing.T) {
	var r StopReviewer = newDeterministicReviewer(defaultArtifactMaxExtends)
	if _, ok := r.(deterministicReviewer); !ok {
		t.Fatalf("corpus must exercise the real deterministicReviewer, got %T", r)
	}
}

func TestStopReview_RenderWedgeOverride(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault, // boot
		"⏺ working on it…",      // post-paste baseline (non-blank)
		"",                      // every subsequent capture: blank (live session, render wedge)
	}}}
	rev := &scriptedReviewer{}
	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		ArtifactTimeoutS: 2,
		Reviewer:         rev,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr.String())
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	ev := rev.events[0]
	if ev.State != panestream.LivenessBusyButStagnant {
		t.Fatalf("StopEvent.State = %v at the first blank-live checkpoint, want LivenessBusyButStagnant (cycle-291 render-wedge override lost)", ev.State)
	}
	if !ev.Busy {
		t.Fatal("StopEvent.Busy = false, want true — a blank pane from a live session must never read Busy=false (center.Busy(session) || renderWedged must stay true)")
	}
}
