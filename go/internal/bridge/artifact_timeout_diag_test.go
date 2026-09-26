package bridge

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

func TestRunTmuxREPL_ArtifactTimeout_SummaryCarriesWaitedAndExtends(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never lands
	// Two extends then a pause, so extends_used is 2 and the pause reason must survive.
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
		"cause=review_pause",
		"phase=audit",
		"cycle=0",
		"driver=claude-tmux",
		`artifact="artifact.md"`,
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
	// A 2s interval and three review checkpoints mean the driver waited at least 4s.
	if strings.Contains(summary, "waited=0s") {
		t.Errorf("waited=0s after three review intervals — the elapsed wait is not being recorded\n  got: %s", summary)
	}
	if got := strings.Count(stderr, artifactTimeoutMarker); got != 1 {
		t.Fatalf("timeout marker count = %d, want exactly one; stderr=%q", got, stderr)
	}
	markerAt := strings.Index(stderr, artifactTimeoutMarker)
	for _, diagnostic := range []string{"FAIL: completion never signalled", "diagnostic: files present under workspace"} {
		if at := strings.Index(stderr, diagnostic); at < 0 || at > markerAt {
			t.Errorf("%q was not emitted before the authoritative marker; stderr=%q", diagnostic, stderr)
		}
	}
}

func TestRunTmuxREPL_NegativeMaxExtendsReportsDefault(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "stop"}}}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, reviewer, nil,
		Deps{ArtifactTimeoutS: 2, ArtifactMaxExtends: -1}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr)
	}
	if summary := artifactTimeoutSummary(stderr); !strings.Contains(summary, "max_extends=6") {
		t.Fatalf("negative extension policy did not resolve to the default backstop; summary=%q", summary)
	}
}

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
	if strings.Contains(got, "files present under workspace") {
		t.Errorf("exit-81 error cause is the workspace file listing, not the timeout summary\n  got: %s", got)
	}
}

// panestream.LivenessState has no String method, so the summary publishes its own closed word list;
// the zero value is named, never blank.
func TestTimeoutSummaryVocabulary(t *testing.T) {
	for _, tc := range []struct {
		in   panestream.LivenessState
		want string
	}{
		{panestream.LivenessIdle, "idle"},
		{panestream.LivenessBusyButStagnant, "busy_stagnant"},
		{panestream.LivenessConverging, "converging"},
		{panestream.LivenessHung, "hung"},
		{panestream.LivenessExhausted, "exhausted"},
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
		// The wait ended before any checkpoint (ctx cancel); a blank would read as a missing measurement.
		{"", "none"},
	} {
		if got := reviewActionOrNone(tc.in); got != tc.want {
			t.Errorf("reviewActionOrNone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// chattyTimeoutDriver reproduces a timed-out launch's real stderr: [bridge] chatter before the wait, the
// driver's own diagnostics, then the marker summary last. It is its own CLI so no other driver's tests see it.
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
