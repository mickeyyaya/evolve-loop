package bridge

// fresh_session_test.go — F31 (2026-09-26): a fatal-pane fast-fail whose typed
// cause is SESSION-recoverable (the REPL process is gone; the CLI and account
// are fine) gets ONE fresh session of the same CLI before the caller's chain
// walks on. Cycle 1687's triage pane went dead with codex quota-walled and
// ollama unable to write source, so the chain had nowhere to go and the phase
// aborted — a fresh claude session would very likely have finished it.

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

func TestFreshSessionRetry_OneFreshSessionForASessionRecoverableCause(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	e := NewEngine(deps)
	ws := t.TempDir()
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: "triage", Cycle: 1687, RunID: "run-1687", Workspace: ws}
	calls := 0
	run := e.freshSessionRetry(context.Background(), req, "auto", func() launchRun {
		calls++
		if calls == 1 {
			return launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseDeadShell, intervalS: 300}, start: time.Now(), end: time.Now()}
		}
		return launchRun{code: ExitOK}
	})
	if calls != 2 || run.code != ExitOK {
		t.Fatalf("a dead shell gets exactly one fresh session: calls=%d code=%d", calls, run.code)
	}
	w := eventsWithCode(*got, CodeFreshSessionRetry)
	if len(w) != 1 || w[0].Fields["cli"] != "claude-tmux" || w[0].Fields["agent"] != "triage" || w[0].Cycle != 1687 || !strings.Contains(w[0].Reason, string(recovery.CauseDeadShell)) {
		t.Fatalf("one BRIDGE_FRESH_SESSION_RETRY naming the cause and the dispatch: %+v", w)
	}
	rec := lastLedgerRecord(t, ws)
	if rec.CallID == "" || rec.CallID != w[0].Fields["call_id"] || !rec.FreshSessionRetry {
		t.Fatalf("the dead dispatch is on the ledger, marked fresh_session_retry, under the signal's call_id: %+v vs %+v", rec, w[0].Fields)
	}
}

// TestFreshSessionRetry_TheGate pins every refusal of freshSessionAllowed
// (architecture review F31): a model/config cause, no cause, a run that
// delivered (not the fast-fail's exit), a NAMED session (kept alive for
// resume — a re-run would reattach to the dead pane outside the sandbox; the
// driver's own resolution rides the observation), a canceled launch, a
// deadline with no room for another of the driver's waits, and a second
// death (at most one retry).
func TestFreshSessionRetry_TheGate(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	e := NewEngine(deps)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tight, cancelTight := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelTight()
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Cycle: 9, Workspace: t.TempDir()}
	dead := fatalPaneObservation{cause: recovery.CauseDeadShell, intervalS: 300}
	named := dead
	named.named = true
	cases := []struct {
		name  string
		ctx   context.Context
		first launchRun
		want  int
	}{
		{"model/config cause: a fresh session fails the same way", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseModelInvalid, intervalS: 300}}, 1},
		{"no fatal cause at all", context.Background(), launchRun{code: ExitArtifactTimeout}, 1},
		{"a delivered run is never run twice", context.Background(), launchRun{code: ExitOK, fatal: dead}, 1},
		{"a named session belongs to its owner", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: named}, 1},
		{"a canceled launch is never retried", canceled, launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseCLISelfUpdated, intervalS: 300}}, 1},
		{"no room on the deadline for another of the driver's waits", tight, launchRun{code: ExitArtifactTimeout, fatal: dead}, 1},
		{"a second death is final: at most one retry", context.Background(), launchRun{code: ExitArtifactTimeout, fatal: dead}, 2},
	}
	for _, tc := range cases {
		calls := 0
		e.freshSessionRetry(tc.ctx, req, "auto", func() launchRun {
			calls++
			return tc.first
		})
		if calls != tc.want {
			t.Errorf("%s: calls=%d, want %d", tc.name, calls, tc.want)
		}
	}
	if w := eventsWithCode(*got, CodeFreshSessionRetry); len(w) != 1 {
		t.Errorf("only the second-death case retried once: %d retry signals", len(w))
	}
}

// TestRunTmuxREPL_FatalCheckpointReportsItsTypedCause: the wait loop hands the
// preempting fatal verdict's typed cause to the call-local observer — the one
// channel the engine's fresh-session retry reads — exactly once.
func TestRunTmuxREPL_FatalCheckpointReportsItsTypedCause(t *testing.T) {
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

// scriptedSessionTmux is the cycle-1687 shape end to end: the first `dead`
// sessions' REPLs boot, take the prompt and die to a bare shell echoing the
// nudge; every later session is a healthy REPL whose prompt delivery writes
// the artifact.
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
	resp, err := newTestEngine(deps).Launch(context.Background(), req)
	rows, readErr := llmcalls.ReadWorkspace(fx.ws)
	if readErr != nil {
		t.Fatalf("read the ledger: %v", readErr)
	}
	return tmux, resp, err, &rows, len(eventsWithCode(*got, CodeFreshSessionRetry))
}

// TestEngineLaunch_DeadShellGetsOneFreshSessionAndSucceeds: the dead pane's
// cause reaches the engine, which runs ONE fresh session that succeeds — the
// launch returns OK instead of exit 81 into a chain with nowhere to go.
func TestEngineLaunch_DeadShellGetsOneFreshSessionAndSucceeds(t *testing.T) {
	tmux, resp, err, rows, retries := launchScripted(t, 1, "enforce", nil)
	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("the fresh session completes the phase: code=%d err=%v", resp.ExitCode, err)
	}
	if tmux.sessions != 2 || retries != 1 {
		t.Fatalf("exactly one fresh session after the dead one: sessions=%d retry signals=%d", tmux.sessions, retries)
	}
	if len(rows.Records) != 2 || !rows.Records[0].FreshSessionRetry || rows.Records[1].FreshSessionRetry || rows.Records[0].Attempt != rows.Records[1].Attempt {
		t.Fatalf("two ledger rows sharing the attempt, the dead one marked: %+v", rows.Records)
	}
}

// TestEngineLaunch_ASecondDeathReturnsExit81ToTheChain: the fresh session dies
// too — the launch returns exit 81 (the chain walks as before), with two rows
// and one retry signal: at most one retry.
func TestEngineLaunch_ASecondDeathReturnsExit81ToTheChain(t *testing.T) {
	tmux, resp, err, rows, retries := launchScripted(t, 2, "enforce", nil)
	if err == nil || resp.ExitCode != ExitArtifactTimeout || !strings.Contains(err.Error(), "exit=81") {
		t.Fatalf("a second death returns exit 81 to the chain: code=%d err=%v", resp.ExitCode, err)
	}
	if tmux.sessions != 2 || retries != 1 || len(rows.Records) != 2 || !rows.Records[0].FreshSessionRetry || rows.Records[1].FreshSessionRetry {
		t.Fatalf("one retry, two rows, the dead first marked: sessions=%d retries=%d rows=%+v", tmux.sessions, retries, rows.Records)
	}
}

// TestEngineLaunch_NoFreshSessionWithoutEnforceOrForANamedSession: at shadow the
// fast-fail never preempts, so no cause is reported and no retry runs; a named
// session is never retried (it would reattach to the dead pane).
func TestEngineLaunch_NoFreshSessionWithoutEnforceOrForANamedSession(t *testing.T) {
	for _, tc := range []struct {
		name, stage string
		mutate      func(*core.BridgeRequest)
	}{
		{"shadow stage", "shadow", nil},
		{"named by the request", "enforce", func(r *core.BridgeRequest) { r.SessionName = "swarm-c1687-w1" }},
		// Architecture re-review N1: the session name can come from the env
		// (or a profile) — the driver's resolveSession decides named-ness, and
		// the gate reads the driver's answer, never the request field.
		{"named by BRIDGE_SESSION_NAME", "enforce", func(r *core.BridgeRequest) { r.Env = map[string]string{"BRIDGE_SESSION_NAME": "swarm-c1687-w1"} }},
	} {
		tmux, resp, _, rows, retries := launchScripted(t, 2, tc.stage, tc.mutate)
		if resp.ExitCode != ExitArtifactTimeout || tmux.sessions != 1 || retries != 0 || len(rows.Records) != 1 {
			t.Errorf("%s: no retry — code=%d sessions=%d retries=%d rows=%d", tc.name, resp.ExitCode, tmux.sessions, retries, len(rows.Records))
		}
	}
}

// TestFreshSessionRetry_ClearsTheDeadDispatchsBootStrike (re-review N3): the
// dead dispatch BOOTED, so its boot strike is cleared — observable through a
// store whose clear fails, which reports one BRIDGE_BOOT_STRIKE_CLEAR_FAILED
// for the dead dispatch before the fresh session runs.
func TestFreshSessionRetry_ClearsTheDeadDispatchsBootStrike(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	deps.BootTimeoutStore = brokenBootStrikeStore(t)
	e := NewEngine(deps)
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Cycle: 9, Workspace: t.TempDir()}
	calls := 0
	e.freshSessionRetry(context.Background(), req, "auto", func() launchRun {
		calls++
		return launchRun{code: ExitArtifactTimeout, fatal: fatalPaneObservation{cause: recovery.CauseDeadShell, intervalS: 300}}
	})
	if calls != 2 || len(eventsWithCode(*got, CodeBootStrikeClearFailed)) != 1 {
		t.Fatalf("the dead dispatch's boot strike is cleared once before the retry: calls=%d clears=%d", calls, len(eventsWithCode(*got, CodeBootStrikeClearFailed)))
	}
}
