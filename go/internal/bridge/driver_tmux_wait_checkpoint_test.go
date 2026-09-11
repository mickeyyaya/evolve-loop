package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func TestReviewCheckpoint_BuildsAndPublishesEvidence(t *testing.T) {
	workspace := t.TempDir()
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "unit pause"}}}
	var callback stopReviewRec
	var stderr bytes.Buffer
	deps := Deps{
		Stderr:        &stderr,
		Reviewer:      reviewer,
		RecoveryStage: "off",
		OnStopReview: func(phase, action, reason string) {
			callback = stopReviewRec{phase: phase, action: action, reason: reason}
		},
	}.withDefaults()
	profile := panestream.Profiles["claude"]
	waiter := replWaiter{
		ctx: context.Background(),
		cfg: &Config{
			Agent: "build", Cycle: 7, ArtifactTimeoutS: 2,
			Workspace: workspace, ProjectRoot: workspace,
			Artifact: filepath.Join(workspace, "artifact.md"),
		},
		deps:      deps,
		launch:    tmuxLaunch{name: "claude-tmux", session: "checkpoint-unit"},
		prefix:    "[checkpoint-unit]",
		phaseName: "build",
		responder: &autoResponder{injectedPrompt: "write the artifact"},
		recorder:  interaction.NewRecorder(workspace),
		channel:   &replLiveChannel{profile: profile},
	}
	state := newReplWaitState(waiter)
	pane := tmuxPromptMarkerDefault + "\n⏺ current evidence"

	if got := waiter.reviewCheckpoint(state, 6, pane, false); got != checkpointReviewed {
		t.Fatalf("reviewCheckpoint result = %v, want checkpointReviewed", got)
	}
	if len(reviewer.events) != 1 {
		t.Fatalf("reviewer events = %d, want 1", len(reviewer.events))
	}
	event := reviewer.events[0]
	if event.Kind != StopArtifactTimeout || event.Phase != "build" || event.Cycle != 7 ||
		event.ElapsedS != 6 || event.IntervalS != 2 || event.Attempt != 0 {
		t.Errorf("checkpoint event identity/timing = %+v", event)
	}
	if !strings.Contains(event.StdoutTail, "current evidence") || event.InjectedPrompt != "write the artifact" {
		t.Errorf("checkpoint event lost pane/prompt evidence: %+v", event)
	}
	if state.result.lastGoodPane != pane {
		t.Errorf("lastGoodPane = %q, want current pane %q", state.result.lastGoodPane, pane)
	}
	if state.lastVerdict.Action != ReviewPause || callback.action != string(ReviewPause) || callback.reason != "unit pause" {
		t.Errorf("published verdict/callback = %+v / %+v", state.lastVerdict, callback)
	}
}

// TestRunTmuxREPL_PersistentFatalCheckpointPreservesTheWholeDecisionChain
// characterizes the checkpoint boundary before it is extracted from wait().
// A persistent fatal pane must cross the gate once, bypass the reviewer on the
// crossing checkpoint, publish the stop decision, skip the idle nudge, and
// reach the ordinary exit-81 closeout with its evidence intact.
func TestRunTmuxREPL_PersistentFatalCheckpointPreservesTheWholeDecisionChain(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	fatalPane := tmuxPromptMarkerDefault + "\n" + fatalTail + "\n" + tmuxPromptMarkerDefault
	tmux := &fakeTmux{paneSeq: checkpointPaneSeq(fatalPane, fatalPane)}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "first observation has not crossed the persistence gate"},
		{Action: ReviewPause, Reason: "must never be returned: fatal checkpoint bypasses reviewer"},
	}}

	var stderr bytes.Buffer
	var callbacks []stopReviewRec
	fatalRecordedBeforeCallback := false
	stopLoggedBeforeCallback := false
	deps := Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		LookupEnv:        mapLookup(nil),
		CaptureBaseline:  zeroBaselineCapture,
		Reviewer:         reviewer,
		ArtifactTimeoutS: 2,
		RecoveryStage:    "enforce",
	}
	deps.OnStopReview = func(phase, action, reason string) {
		callbacks = append(callbacks, stopReviewRec{phase: phase, action: action, reason: reason})
		if action != string(ReviewStop) {
			return
		}
		stopLoggedBeforeCallback = strings.Contains(stderr.String(), "→ stop:")
		ledger, err := os.ReadFile(filepath.Join(fx.ws, "build-interactions.ndjson"))
		if err != nil {
			return
		}
		fatalRecordedBeforeCallback = strings.Contains(string(ledger), `"kind":"fatal_pane_shadow"`) &&
			strings.Contains(string(ledger), `"result":"fast_failed"`)
	}

	eng := newTestEngine(deps)
	var stdout bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=262"),
		nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout (%d); stderr=%q", code, ExitArtifactTimeout, stderr.String())
	}
	if len(reviewer.events) != 1 {
		t.Fatalf("reviewer calls = %d, want 1 before the fatal gate crosses", len(reviewer.events))
	}
	if len(callbacks) != 2 || callbacks[0].action != string(ReviewExtend) || callbacks[1].action != string(ReviewStop) {
		t.Fatalf("stop-review callbacks = %+v, want extend then fatal stop", callbacks)
	}
	if !fatalRecordedBeforeCallback || !stopLoggedBeforeCallback {
		t.Fatalf("stop callback ran before its evidence/log: fatal_recorded=%v stop_logged=%v stderr=%q",
			fatalRecordedBeforeCallback, stopLoggedBeforeCallback, stderr.String())
	}

	var fatalOutcomes, nudgeOutcomes int
	for _, outcome := range readInteractionLedger(t, fx.ws, "build") {
		switch outcome.Kind {
		case "fatal_pane_shadow":
			fatalOutcomes++
			if outcome.Result != "fast_failed" {
				t.Errorf("fatal outcome result = %q, want fast_failed", outcome.Result)
			}
		case interaction.KindNudge:
			nudgeOutcomes++
		}
	}
	if fatalOutcomes != 1 {
		t.Errorf("fatal outcomes = %d, want exactly 1", fatalOutcomes)
	}
	if nudgeOutcomes != 0 || tmux.sentContains("Please write the deliverable") {
		t.Errorf("fatal stop sent a nudge: outcomes=%d sent=%v", nudgeOutcomes, tmux.sentSeq)
	}

	summary := artifactTimeoutSummary(stderr.String())
	if !strings.Contains(summary, "last_review=stop") || !strings.Contains(summary, "fatal pane state (model_invalid)") {
		t.Errorf("exit-81 summary lost the fatal stop evidence: %q", summary)
	}
}
