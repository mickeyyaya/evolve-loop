package bridge

// artifact_timeout_diag_test.go — an artifact-timeout death must be
// SELF-DESCRIBING.
//
// The defect (inbox item deep-phase-artifact-budget-too-small, sub-task 3): exit
// 81 produced no deliverable and no reason beyond the code, so the reader of a
// dead cycle could not tell "the agent was still working and ran out of budget"
// (raise the budget) from "the pane was wedged" (fix the wedge). Worse, the
// cause line threaded into the error came from firstDiagnosticLine, which — for
// the tmux driver, whose notes are prefixed `[<cli>-tmux]`, not `[bridge]` —
// fell through to the LAST non-empty stderr line: one of the workspace file
// listings the timeout path prints as a diagnostic. The recorded error_message
// was a filename.
//
// Contract: the driver emits ONE self-describing summary line carrying how long
// it waited and how many extends it consumed, and Engine.Launch lifts exactly
// that line into the exit-81 error, deterministically, regardless of what other
// `[bridge]` chatter the launch produced.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestRunTmuxREPL_ArtifactTimeout_SummaryCarriesWaitedAndExtends: the driver's
// stderr must carry the marker line with the elapsed wait and the extends
// consumed, plus the last review verdict that distinguishes slow from wedged.
func TestRunTmuxREPL_ArtifactTimeout_SummaryCarriesWaitedAndExtends(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never lands
	// Two extends then a pause: extends_used must be 2, and the pause reason must
	// survive into the summary.
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "still working"},
		{Action: ReviewExtend, Reason: "still working"},
		{Action: ReviewPause, Reason: "agent busy but produced no output"},
	}}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: 5}, "--allow-bypass", "--agent=audit")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	summary := artifactTimeoutSummary(stderr)
	if summary == "" {
		t.Fatalf("no %q line on stderr — an artifact-timeout death must describe itself; stderr=%q",
			artifactTimeoutMarker, stderr)
	}
	for _, want := range []string{
		"phase=audit",
		"waited=",
		"interval=2s",
		"extends_used=2",
		"max_extends=5",
		"last_review=pause",
		"agent busy but produced no output",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary is missing %q — a reader cannot tell 'too slow' from 'wedged'\n  got: %s", want, summary)
		}
	}
	// The elapsed wait must be a real measurement, not a zero placeholder: with a
	// 2s interval and three review checkpoints the driver waited at least 4s.
	if strings.Contains(summary, "waited=0s") {
		t.Errorf("waited=0s after three review intervals — the elapsed wait is not being recorded\n  got: %s", summary)
	}
}

// TestEngineLaunch_ArtifactTimeout_ErrorCarriesWaitAndExtends is the live-path
// proof: the summary reaches the ERROR the orchestrator records, not just a log
// nobody reads. Driven through the real Engine.Launch → LaunchArgs → tmux driver
// path with a fake tmux, so the assertion is on the production wrapping site.
func TestEngineLaunch_ArtifactTimeout_ErrorCarriesWaitAndExtends(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "still working"},
		{Action: ReviewPause, Reason: "no output during the last 2s interval"},
	}}
	eng := newTestEngine(Deps{
		Tmux:               tmux,
		Sleep:              func(time.Duration) {},
		Reviewer:           rev,
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 4,
		LookupEnv:          mapLookup(nil),
	})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: "x",
		Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"),
		Agent: "audit", PermissionMode: "plan",
	})
	if err == nil {
		t.Fatal("expected an error when the artifact never appears")
	}
	got := err.Error()
	for _, want := range []string{artifactTimeoutMarker, "waited=", "extends_used=1", "max_extends=4", "last_review=pause"} {
		if !strings.Contains(got, want) {
			t.Errorf("exit-81 error is missing %q — the recorded failure reason must say how long it waited "+
				"and how many extends it consumed\n  got: %s", want, got)
		}
	}
	// Regression on the real defect: the cause must not be a workspace file
	// listing line that firstDiagnosticLine happened to land on.
	if strings.Contains(got, "files present under workspace") {
		t.Errorf("exit-81 error cause is the workspace file listing, not the timeout summary\n  got: %s", got)
	}
}

// TestArtifactTimeoutSummary_WinsOverEarlierBridgeChatter: the extractor must be
// marker-driven, not position-driven. A sandbox WARN (`[bridge] WARN: …`) is
// emitted BEFORE the artifact wait on real launches, so a first-`[bridge]`-line
// heuristic would report the sandbox note as the timeout's cause.
func TestArtifactTimeoutSummary_WinsOverEarlierBridgeChatter(t *testing.T) {
	stderr := strings.Join([]string{
		"[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied",
		"[claude-tmux] FAIL: completion never signalled",
		"[claude-tmux]   audit-report.md",
		"[bridge] " + artifactTimeoutMarker + "phase=audit waited=650s extends_used=6",
		"",
	}, "\n")
	got := artifactTimeoutSummary(stderr)
	if !strings.Contains(got, "waited=650s") || !strings.Contains(got, "extends_used=6") {
		t.Errorf("artifactTimeoutSummary = %q, want the marker line with waited/extends", got)
	}
	if strings.Contains(got, "EVOLVE_SANDBOX") {
		t.Errorf("extractor returned the earlier sandbox WARN instead of the timeout summary: %q", got)
	}
	if s := artifactTimeoutSummary("[bridge] no timeout here\n"); s != "" {
		t.Errorf("artifactTimeoutSummary on unrelated stderr = %q, want \"\"", s)
	}
}

// TestTimeoutSummaryVocabulary pins the closed word list the summary publishes
// for the two enum fields. These words are what an operator greps for, and
// panestream.LivenessState has no String method — a %s on it would emit Go debug
// chrome instead of a stable token, and the zero value ("no checkpoint observed
// liveness") must be named rather than blank.
func TestTimeoutSummaryVocabulary(t *testing.T) {
	for _, tc := range []struct {
		in   panestream.LivenessState
		want string
	}{
		{panestream.LivenessIdle, "idle"},
		{panestream.LivenessBusyButStagnant, "busy_stagnant"},
		{panestream.LivenessConverging, "converging"},
		{panestream.LivenessHung, "hung"},
		{0, "unknown"},
	} {
		if got := livenessOrUnknown(tc.in); got != tc.want {
			t.Errorf("livenessOrUnknown(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		in   ReviewAction
		want string
	}{
		{ReviewExtend, "extend"},
		{ReviewPause, "pause"},
		{ReviewStop, "stop"},
		// The wait ended before any review checkpoint (ctx cancel) — a blank
		// field would read as a missing measurement rather than a real state.
		{"", "none"},
	} {
		if got := reviewActionOrNone(tc.in); got != tc.want {
			t.Errorf("reviewActionOrNone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// chattyTimeoutDriver reproduces the REAL stderr shape of a timed-out launch:
// `[bridge]` chatter emitted BEFORE the artifact wait (a sandbox WARN, a codex
// preflight note), then the driver's own `[<cli>-tmux]`-prefixed diagnostics
// including the workspace file listing, then the marker summary LAST. Registered
// as its own CLI so it cannot perturb any other driver's tests.
type chattyTimeoutDriver struct{}

func (chattyTimeoutDriver) Name() string { return "acs-chatty-timeout" }

func (chattyTimeoutDriver) Launch(_ context.Context, cfg *Config, deps Deps) (int, error) {
	fmt.Fprintln(deps.Stderr, `[bridge] WARN: EVOLVE_SANDBOX="bogus" unrecognized (want auto|on|off); treating as auto`)
	fmt.Fprintln(deps.Stderr, "[acs-chatty-timeout] FAIL: completion never signalled (artifact "+cfg.Artifact+")")
	fmt.Fprintln(deps.Stderr, "[acs-chatty-timeout] diagnostic: files present under workspace "+cfg.Workspace+":")
	fmt.Fprintln(deps.Stderr, "[acs-chatty-timeout]   audit-prompt.txt")
	fmt.Fprintf(deps.Stderr, "[bridge] %sphase=audit waited=650s interval=300s extends_used=6 max_extends=6 "+
		"last_review=pause liveness=busy_stagnant progressed=false busy=true reason=%q\n",
		artifactTimeoutMarker, "agent busy but produced no output — exhausted 6 extensions")
	return ExitArtifactTimeout, nil
}

func init() { Register(chattyTimeoutDriver{}) }

// TestEngineLaunch_ArtifactTimeout_SummaryBeatsEarlierBridgeChatter is the
// discriminative proof that the engine's extractor is load-bearing rather than
// incidental: firstDiagnosticLine returns the FIRST `[bridge]`-prefixed line, so
// with a sandbox WARN (or a codex preflight note) ahead of the wait the recorded
// cause is that WARN — a launch-time note that says nothing about why the phase
// died. Driven through the real Engine.Launch wrapping site.
func TestEngineLaunch_ArtifactTimeout_SummaryBeatsEarlierBridgeChatter(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "chatty", "")
	eng := NewEngine(Deps{LookupEnv: mapLookup(nil)})
	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "acs-chatty-timeout", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "audit",
	})
	if err == nil {
		t.Fatal("expected an error on exit 81")
	}
	got := err.Error()
	if !strings.Contains(got, "waited=650s") || !strings.Contains(got, "extends_used=6") {
		t.Errorf("exit-81 error lost the timeout summary to earlier chatter\n  got: %s", got)
	}
	if strings.Contains(got, "EVOLVE_SANDBOX") {
		t.Errorf("exit-81 cause is the launch-time sandbox WARN, not the timeout summary\n  got: %s", got)
	}
	if strings.Contains(got, "files present under workspace") {
		t.Errorf("exit-81 cause is the workspace file listing, not the timeout summary\n  got: %s", got)
	}
}

// TestEngineLaunch_NonTimeoutExit_CauseUnchanged: the summary preference is
// SCOPED to exit 81. Any other non-zero exit keeps the existing
// firstDiagnosticLine cause, so this fix cannot degrade unrelated diagnostics.
func TestEngineLaunch_NonTimeoutExit_CauseUnchanged(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-test", "")
	fr := &fakeRunner{exit: ExitSafetyGate}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})
	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Agent: "build-planner",
	})
	if err == nil {
		t.Fatal("expected an error on exit 2")
	}
	if strings.Contains(err.Error(), artifactTimeoutMarker) {
		t.Errorf("a non-81 exit must not carry the artifact-timeout summary; got %v", err)
	}
}
