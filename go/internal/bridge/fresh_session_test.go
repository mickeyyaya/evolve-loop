package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

const deadShellPaneLine = "zsh: command not found: Please"

func TestFreshSessionAllowed_TheGate(t *testing.T) {
	t.Parallel()
	e := NewEngine(Deps{})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tight, cancelTight := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelTight()
	roomy, cancelRoomy := context.WithTimeout(context.Background(), time.Hour)
	defer cancelRoomy()
	dead := fatalPaneObservation{cause: recovery.CauseDeadShell, intervalS: 300}
	named := dead
	named.named = true
	cases := []struct {
		name  string
		ctx   context.Context
		first launchRun
		want  bool
	}{
		{"model/config cause: a fresh session fails the same way", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseModelInvalid, intervalS: 300}}, false},
		{"no fatal cause at all", context.Background(), launchRun{code: ExitArtifactTimeout}, false},
		{"a delivered run is never run twice", context.Background(), launchRun{code: ExitOK, fatal: dead}, false},
		{"a named session belongs to its owner", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: named}, false},
		{"a canceled launch is never retried", canceled, launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseCLISelfUpdated, intervalS: 300}}, false},
		{"no room on the deadline for another of the driver's waits", tight, launchRun{code: ExitArtifactTimeout, fatal: dead}, false},
		{"room on the deadline", roomy, launchRun{code: ExitArtifactTimeout, fatal: dead}, true},
		{"a dead shell with no deadline", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: dead}, true},
	}
	for _, tc := range cases {
		if got := e.freshSessionAllowed(tc.ctx, tc.first); got != tc.want {
			t.Errorf("%s: freshSessionAllowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRunTmuxREPL_FatalCheckpointReportsItsTypedCause(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	deadPane := tmuxPromptMarkerDefault + "\n" + deadShellPaneLine + "\n" + tmuxPromptMarkerDefault
	tmux := &fakeTmux{paneSeq: checkpointPaneSeq(deadPane, deadPane)}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "first observation has not crossed the persistence gate"},
		{Action: ReviewPause, Reason: "must never be returned: the fatal checkpoint bypasses the reviewer"},
	}}
	var observations []fatalPaneObservation
	deps := Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		LookupEnv:        mapLookup(nil),
		CaptureBaseline:  zeroBaselineCapture,
		Reviewer:         reviewer,
		ArtifactTimeoutS: 2,
		FatalPaneStage:   "enforce",
		onFatalPane:      func(o fatalPaneObservation) { observations = append(observations, o) },
	}
	var stdout, stderr bytes.Buffer
	code := newTestEngine(deps).LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=triage", "--cycle=1687"), nil, &stdout, &stderr)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr.String())
	}
	if len(observations) != 1 || observations[0] != (fatalPaneObservation{cause: recovery.CauseDeadShell, named: false, intervalS: 2}) {
		t.Fatalf("the preempting verdict's cause, the ephemeral session and the interval used are reported exactly once: %+v", observations)
	}
}

// scriptedSessionTmux: the first `dead` sessions' REPLs boot, take the prompt, and die to a bare shell
// echoing the nudge; every later session is healthy and its prompt delivery writes the artifact.
type scriptedSessionTmux struct {
	*fakeTmux
	sessions, dead int
	artifact       string
}

func (d *scriptedSessionTmux) NewSession(ctx context.Context, name string, w, h int) error {
	d.sessions++
	return d.fakeTmux.NewSession(ctx, name, w, h)
}

func (d *scriptedSessionTmux) CapturePane(context.Context, string, int) (string, error) {
	if d.sessions <= d.dead {
		return tmuxPromptMarkerDefault + "\n" + deadShellPaneLine + "\n" + tmuxPromptMarkerDefault, nil
	}
	return tmuxPromptMarkerDefault, nil
}

func (d *scriptedSessionTmux) PasteBuffer(context.Context, string) error {
	if d.sessions > d.dead {
		return os.WriteFile(d.artifact, []byte("done\n"), 0o644)
	}
	return nil
}

// launchScripted drives the real Engine.Launch → runScoped → claude-tmux driver
// → checkpoint path against scriptedSessionTmux.
func launchScripted(t *testing.T, dead int, stage string, mutate func(*core.BridgeRequest)) (*scriptedSessionTmux, core.BridgeResponse, error, *llmcalls.ReadResult, int) {
	t.Helper()
	return launchScriptedWith(t, dead, stage, mutate, func(*Deps) {})
}

func launchScriptedWith(t *testing.T, dead int, stage string, mutate func(*core.BridgeRequest), wire func(*Deps)) (*scriptedSessionTmux, core.BridgeResponse, error, *llmcalls.ReadResult, int) {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	artifact := filepath.Join(fx.ws, "triage-report.md")
	tmux := &scriptedSessionTmux{fakeTmux: &fakeTmux{}, dead: dead, artifact: artifact}
	deps, got, _ := recordingDeps(t)
	deps.Tmux = tmux
	deps.Sleep = func(time.Duration) {}
	deps.Reviewer = &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewExtend, Reason: "first observation has not crossed the persistence gate"}}}
	deps.ArtifactTimeoutS = 2
	deps.FatalPaneStage = stage
	deps.LookupEnv = mapLookup(nil)
	req := core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: "x",
		Workspace: fx.ws, ArtifactPath: artifact, Agent: "triage", Cycle: 1687, RunID: "run-1687",
	}
	if mutate != nil {
		mutate(&req)
	}
	wire(&deps)
	resp, err := newTestEngine(deps).Launch(context.Background(), req)
	rows, readErr := llmcalls.ReadWorkspace(fx.ws)
	if readErr != nil {
		t.Fatalf("read the ledger: %v", readErr)
	}
	return tmux, resp, err, &rows, len(eventsWithCode(*got, CodeFreshSessionRetry))
}

func TestEngineLaunch_ADeadShellAsksTheCallerForAFreshSessionAndRunsNoSecondSession(t *testing.T) {
	t.Parallel()
	tmux, resp, err, rows, retries := launchScripted(t, 1, "enforce", nil)
	if err == nil || resp.ExitCode != ExitArtifactTimeout || !resp.SessionRecoverable {
		t.Fatalf("the dead session returns exit 81 marked session-recoverable: code=%d recoverable=%v err=%v", resp.ExitCode, resp.SessionRecoverable, err)
	}
	if tmux.sessions != 1 || retries != 0 {
		t.Fatalf("the bridge runs no second session and claims no retry: sessions=%d retry signals=%d", tmux.sessions, retries)
	}
	if len(rows.Records) != 1 || !rows.Records[0].SessionRecoverable || rows.Records[0].FreshSessionRetry {
		t.Fatalf("one ledger row, marked session_recoverable and not fresh_session_retry: %+v", rows.Records)
	}
}

func TestEngineLaunch_TheFreshSessionIsMarkedAndSignalledWhenItLaunches(t *testing.T) {
	t.Parallel()
	_, resp, err, rows, retries := launchScripted(t, 0, "enforce", func(r *core.BridgeRequest) { r.FreshSession = true })
	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("the fresh session completes: code=%d err=%v", resp.ExitCode, err)
	}
	if retries != 1 || len(rows.Records) != 1 || !rows.Records[0].FreshSessionRetry || rows.Records[0].SessionRecoverable {
		t.Fatalf("retry signals=%d rows=%+v, want one BRIDGE_FRESH_SESSION_RETRY and the row marked fresh_session_retry", retries, rows.Records)
	}
}

func TestEngineLaunch_TheResponseNamesTheDispatchAndItsOwnPaneCapture(t *testing.T) {
	t.Parallel()
	_, resp, _, _, _ := launchScripted(t, 1, "enforce", nil)
	if !strings.HasPrefix(resp.DispatchID, "run-1687/1687/triage/") {
		t.Fatalf("DispatchID = %q, want the dispatch tag of this launch", resp.DispatchID)
	}
	base := filepath.Base(resp.PaneCapture)
	if !strings.HasPrefix(base, "triage-") || !strings.HasSuffix(base, "-final-pane.txt") {
		t.Fatalf("PaneCapture = %q, want <workspace>/triage-<session>-final-pane.txt", resp.PaneCapture)
	}
	body, err := os.ReadFile(resp.PaneCapture)
	if err != nil || !strings.Contains(string(body), deadShellPaneLine) {
		t.Fatalf("the capture = %q, %v; want the final pane of this dispatch", body, err)
	}
}

func TestEngineLaunch_NoFreshSessionWithoutEnforceOrForANamedSession(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stage string
		mutate      func(*core.BridgeRequest)
	}{
		{"shadow stage", "shadow", nil},
		{"named by the request", "enforce", func(r *core.BridgeRequest) { r.SessionName = "swarm-c1687-w1" }},
		{"named by BRIDGE_SESSION_NAME", "enforce", func(r *core.BridgeRequest) { r.Env = map[string]string{"BRIDGE_SESSION_NAME": "swarm-c1687-w1"} }},
	} {
		tmux, resp, _, rows, retries := launchScripted(t, 2, tc.stage, tc.mutate)
		if resp.ExitCode != ExitArtifactTimeout || resp.SessionRecoverable || tmux.sessions != 1 || retries != 0 || len(rows.Records) != 1 || rows.Records[0].SessionRecoverable {
			t.Errorf("%s: not recoverable — code=%d recoverable=%v sessions=%d retries=%d rows=%d", tc.name, resp.ExitCode, resp.SessionRecoverable, tmux.sessions, retries, len(rows.Records))
		}
	}
}

func TestEngineLaunch_TheCallLocalHooksChainThePreWiredObservers(t *testing.T) {
	t.Parallel()
	var dispatched []modelDispatch
	var fatal []fatalPaneObservation
	wire := func(d *Deps) {
		d.onModelDispatch = func(o modelDispatch) { dispatched = append(dispatched, o) }
		d.onFatalPane = func(o fatalPaneObservation) { fatal = append(fatal, o) }
	}

	_, resp, _, _, _ := launchScriptedWith(t, 1, "enforce", nil, wire)

	if len(dispatched) == 0 || len(fatal) != 1 || fatal[0].cause != recovery.CauseDeadShell || !resp.SessionRecoverable {
		t.Fatalf("dispatched = %d fatal = %+v recoverable = %v; want the pre-wired observers called and the response still fed", len(dispatched), fatal, resp.SessionRecoverable)
	}
}
