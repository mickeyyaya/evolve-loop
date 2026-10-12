package bridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

func TestAgyTmuxLaunchBackfillF1_ATargetWithoutAManifestFailsBeforeAnyPaneWork(t *testing.T) {
	t.Parallel()
	tm := &FakeTmuxController{}
	var stderr bytes.Buffer
	deps := Deps{Tmux: tm, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr}.withDefaults()
	cfg := &Config{Workspace: t.TempDir(), AllowBypass: true, Agent: "f1"}
	code, err := agyTmuxDriver{target: "no-such-agy-target"}.Launch(context.Background(), cfg, deps)
	if code != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "[no-such-agy-target] launch model check") {
		t.Fatalf("Launch = (%d, %v), want ExitBadFlags with the launch-model-check failure", code, err)
	}
	if len(tm.SentKeys) != 0 || len(tm.Events) != 0 {
		t.Fatalf("a refused launch touched tmux: keys=%v events=%v", tm.SentKeys, tm.Events)
	}
}

func TestAgyTmuxLaunchBackfillF1_ThePreflightSafetyGateStillComesFirst(t *testing.T) {
	t.Parallel()
	deps := Deps{Tmux: &FakeTmuxController{}, Stderr: io.Discard, LookupEnv: mapLookup(nil)}.withDefaults()
	code, err := agyTmuxDriver{target: "no-such-agy-target"}.Launch(context.Background(), &Config{Workspace: t.TempDir()}, deps)
	if code != ExitSafetyGate || err != nil {
		t.Fatalf("Launch without --allow-bypass = (%d, %v), want (ExitSafetyGate, nil)", code, err)
	}
}

func TestOllamaTmuxLaunchBackfillF1_ABootOnlyLaunchSendsTheComposedRunLineAndBootsOnTheReplPrompt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		model    string
		wantLine string
	}{
		{name: "default model when none is pinned", model: "", wantLine: "ollama run llama3.1:8b --experimental-yolo"},
		{name: "pinned cloud model", model: "gpt-oss:120b-cloud", wantLine: "ollama run gpt-oss:120b-cloud --experimental-yolo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			frames := make([]string, 12)
			for i := range frames {
				frames[i] = ">>> "
			}
			tm := &FakeTmuxController{CaptureFrames: frames}
			deps := Deps{Tmux: tm, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}), Stderr: io.Discard}.withDefaults()
			ws := t.TempDir()
			cfg := &Config{
				Workspace: ws, ProjectRoot: ws, Agent: "f1-ollama", AllowBypass: true, BootOnly: true, Model: tc.model,
				Realization: RealizeFor("ollama-tmux", LaunchIntent{Permission: "bypass"}),
			}
			code, err := ollamaTmuxDriver{}.Launch(context.Background(), cfg, deps)
			if err != nil || code != ExitOK {
				t.Fatalf("Launch = (%d, %v), want (ExitOK, nil)", code, err)
			}
			found := false
			for _, sent := range tm.SentKeys {
				if sent == tc.wantLine {
					found = true
				}
			}
			if !found {
				t.Fatalf("no launch line %q reached the pane; sent=%v", tc.wantLine, tm.SentKeys)
			}
			if !slices.Contains(tm.SentSeq, "/bye|true") {
				t.Fatalf("the REPL must be left with a committed /bye; sent=%v", tm.SentSeq)
			}
		})
	}
}

func TestOllamaTmuxLaunchBackfillF1_ThePreflightGateRefusesWithoutBypass(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	deps := Deps{Tmux: &FakeTmuxController{}, Stderr: &stderr, LookupEnv: mapLookup(nil)}.withDefaults()
	code, err := ollamaTmuxDriver{}.Launch(context.Background(), &Config{Workspace: t.TempDir()}, deps)
	if code != ExitSafetyGate || err != nil || !strings.Contains(stderr.String(), "--allow-bypass is required") {
		t.Fatalf("Launch = (%d, %v) stderr=%q", code, err, stderr.String())
	}
}

type idleErrDetectorF1 struct{ err error }

func (idleErrDetectorF1) poll(context.Context) (bool, completionEvidence, string, error) {
	return false, completionEvidence{}, "", nil
}

func (d idleErrDetectorF1) completeOnIdle(context.Context) (bool, completionEvidence, string, error) {
	return false, completionEvidence{}, "", d.err
}

func TestReplWaiterCompleteOnIdleBackfillF1_ADetectorErrorIsLoggedOnceAndNeverCompletes(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	w := replWaiter{ctx: context.Background(), deps: Deps{Stderr: &stderr}, prefix: "[f1]"}
	state := &replWaitState{detector: idleErrDetectorF1{err: errors.New("git went away")}}
	for i := 0; i < 3; i++ {
		if w.completeOnIdle(state) {
			t.Fatal("a detector error must not complete the wait")
		}
	}
	if state.completed {
		t.Fatal("state.completed set by a failing detector")
	}
	if got := strings.Count(stderr.String(), "[f1] WARN: completion detector: git went away"); got != 1 {
		t.Fatalf("warning logged %d times, want exactly once; stderr=%q", got, stderr.String())
	}
	if state.lastDetectorErr == nil || !state.terminalDetectorErrored {
		t.Fatalf("the error must be recorded on the state: %+v", state)
	}
}

func TestReplWaiterCompleteOnIdleBackfillF1_ADetectorWithoutAnIdleHookNeverCompletes(t *testing.T) {
	t.Parallel()
	w := replWaiter{ctx: context.Background(), deps: Deps{Stderr: io.Discard}}
	if w.completeOnIdle(&replWaitState{detector: pollOnlyDetectorF1{}}) {
		t.Fatal("a poll-only detector has no idle hook to complete on")
	}
}

type pollOnlyDetectorF1 struct{}

func (pollOnlyDetectorF1) poll(context.Context) (bool, completionEvidence, string, error) {
	return false, completionEvidence{}, "", nil
}

func TestEscalationPatternBackfillF1(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body, want string
		write            bool
	}{
		{name: "missing report", write: false, want: ""},
		{name: "malformed json", write: true, body: "{not json", want: ""},
		{name: "json without the pattern", write: true, body: `{"phase":"build"}`, want: ""},
		{name: "named pattern", write: true, body: `{"pattern_name":"quota-wall","phase":"build"}`, want: "quota-wall"},
		{name: "unicode pattern", write: true, body: `{"pattern_name":"クォータ"}`, want: "クォータ"},
	}
	for _, tc := range cases {
		ws := t.TempDir()
		if tc.write {
			if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := EscalationPattern(ws); got != tc.want {
			t.Errorf("%s: EscalationPattern = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNowOfBackfillF1(t *testing.T) {
	t.Parallel()
	pinned := time.Unix(1_700_000_000, 0)
	if got := nowOf(Deps{Now: func() time.Time { return pinned }}); !got.Equal(pinned) {
		t.Fatalf("nowOf with a clock = %v, want %v", got, pinned)
	}
	before := time.Now()
	got := nowOf(Deps{})
	if got.Before(before) || got.After(time.Now()) {
		t.Fatalf("nowOf without a clock = %v, want the wall clock", got)
	}
}

func blockedSnapshotF1(t *testing.T, agent string) string {
	t.Helper()
	ws := t.TempDir()
	path := panewatch.Path(ws, agent)
	if err := os.MkdirAll(filepath.Join(path, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestPaneWatcherCloseBackfillF1(t *testing.T) {
	t.Parallel()
	t.Run("a nil watcher is a no-op", func(t *testing.T) {
		t.Parallel()
		var p *paneWatcher
		p.close()
	})
	t.Run("a snapshot that cannot be removed warns and names the consequence", func(t *testing.T) {
		t.Parallel()
		ws := blockedSnapshotF1(t, "f1-agent")
		var stderr bytes.Buffer
		p := &paneWatcher{tracker: panewatch.NewTracker(panewatch.Snapshot{Agent: "f1-agent"}), workspace: ws, stderr: &stderr, prefix: "[f1]"}
		p.close()
		if got := stderr.String(); !strings.Contains(got, "[f1] WARN pane watch:") || !strings.Contains(got, "a stale snapshot stays until the next launch of this agent") {
			t.Fatalf("stderr = %q", got)
		}
	})
	t.Run("an absent snapshot is silent", func(t *testing.T) {
		t.Parallel()
		var stderr bytes.Buffer
		p := &paneWatcher{tracker: panewatch.NewTracker(panewatch.Snapshot{Agent: "f1-agent"}), workspace: t.TempDir(), stderr: &stderr, prefix: "[f1]"}
		p.close()
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want none", stderr.String())
		}
	})
}

func TestEngineRunAttemptBackfillF1(t *testing.T) {
	t.Parallel()
	t.Run("a previous snapshot that cannot be removed is a warning, not a stop", func(t *testing.T) {
		t.Parallel()
		ws := blockedSnapshotF1(t, "f1-agent")
		var stderr bytes.Buffer
		e := NewEngine(Deps{Stderr: &stderr, Stdout: io.Discard, LookupEnv: mapLookup(nil), Sleep: func(time.Duration) {}})
		run := e.runAttempt(context.Background(), core.BridgeRequest{Workspace: ws, Agent: "f1-agent"}, nil)
		if !strings.Contains(stderr.String(), "[bridge] WARN pane watch: a previous attempt's snapshot could not be removed") {
			t.Fatalf("stderr = %q", stderr.String())
		}
		if run.code == ExitOK {
			t.Fatalf("an attempt with no launch args must not report success: %+v", run)
		}
	})
	t.Run("a stale snapshot is removed before the attempt", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		if err := panewatch.Write(ws, panewatch.Snapshot{Agent: "f1-agent"}); err != nil {
			t.Fatal(err)
		}
		e := NewEngine(Deps{Stderr: io.Discard, Stdout: io.Discard, LookupEnv: mapLookup(nil), Sleep: func(time.Duration) {}})
		e.runAttempt(context.Background(), core.BridgeRequest{Workspace: ws, Agent: "f1-agent"}, nil)
		if _, err := os.Stat(panewatch.Path(ws, "f1-agent")); !os.IsNotExist(err) {
			t.Fatalf("stale snapshot survived the new attempt: %v", err)
		}
	})
}

func TestDefaultWallCorroboratorBackfillF1_NilSeamsDegradeToSafeDefaults(t *testing.T) {
	t.Parallel()
	corroborate := DefaultWallCorroborator(nil, nil)
	if !corroborate(context.Background(), "cli-with-no-probe-recipe") {
		t.Fatal("a CLI with no probe recipe cannot be corroborated, so the wall must stand")
	}
	failing := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 1, nil
	}
	if !DefaultWallCorroborator(failing, nil)(context.Background(), "claude-tmux") {
		t.Fatal("a failing probe with no log writer must still corroborate the wall")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if !DefaultWallCorroborator(nil, nil)(cancelled, "claude-tmux") {
		t.Fatal("the default runner must fail a probe whose context is already over, which corroborates the wall")
	}
	healthy := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, nil
	}
	if DefaultWallCorroborator(healthy, nil)(context.Background(), "claude-tmux") {
		t.Fatal("a healthy probe is proof of service, so the wall is not corroborated")
	}
}
