package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type seatJourney struct {
	ws      string
	runner  *fakeRunner
	engine  *Engine
	signals *signalcenter.Center
	events  *[]signalcenter.Event
}

func newSeatJourney(t *testing.T) seatJourney {
	t.Helper()
	signals, events := recordingSignals()
	deps := Deps{LookupEnv: mapLookup(nil), Stderr: io.Discard, Signals: signals, EnvStage: "enforce", SandboxMode: "on"}
	runner := &fakeRunner{}
	deps.Runner = runner.runner()
	deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	return seatJourney{ws: t.TempDir(), runner: runner, engine: NewEngine(deps), signals: signals, events: events}
}

func (j seatJourney) launch(t *testing.T, mutate func(*core.BridgeRequest)) (core.BridgeResponse, error) {
	t.Helper()
	req := core.BridgeRequest{
		CLI: "claude-p", Profile: writeProfile(t, j.ws, "seat", ""), Model: "auto", Prompt: "x",
		Workspace: j.ws, ArtifactPath: filepath.Join(j.ws, "a.md"), Agent: "scout", Worktree: t.TempDir(), ProjectRoot: t.TempDir(),
	}
	if mutate != nil {
		mutate(&req)
	}
	return j.engine.Launch(context.Background(), req)
}

func (j seatJourney) record(t *testing.T) llmcalls.Record {
	t.Helper()
	read, err := llmcalls.ReadWorkspace(j.ws)
	if err != nil || len(read.Records) == 0 {
		t.Fatalf("records=%d err=%v", len(read.Records), err)
	}
	return read.Records[len(read.Records)-1]
}

func (j seatJourney) callEnv(t *testing.T) []string {
	t.Helper()
	if len(j.runner.calls) != 1 {
		t.Fatalf("runner calls=%d", len(j.runner.calls))
	}
	return j.runner.calls[0].env
}

func TestSeatJourney_AHolderTokenGivesTheSeatShapeAndTheRecordSaysIsolated(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken+"\n", 0o600)
	j := newSeatJourney(t)
	resp, err := j.launch(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := j.callEnv(t)
	if dir, _ := seatEnvValue(env, "CLAUDE_CONFIG_DIR"); dir != paths.SeatRoot(home) {
		t.Errorf("CLAUDE_CONFIG_DIR=%q", dir)
	}
	if tok, _ := seatEnvValue(env, "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken {
		t.Error("the token is not in the env of the seat")
	}
	if rec := j.record(t); rec.SeatLogin != "isolated" || resp.SeatLogin != "isolated" {
		t.Errorf("record %q response %q", rec.SeatLogin, resp.SeatLogin)
	}
	if !strings.Contains(strings.Join(j.runner.calls[0].args, " "), "sandbox-") && j.runner.calls[0].name != "sandbox-exec" {
		t.Errorf("the token seat ran unconfined: %s %v", j.runner.calls[0].name, j.runner.calls[0].args)
	}
	assertNoCanary(t, j)
}

func TestSeatJourney_ANoTokenHostGivesTheStoredShapeAndTheRecordSaysStored(t *testing.T) {
	seatHome(t)
	j := newSeatJourney(t)
	resp, err := j.launch(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := j.callEnv(t)
	for _, name := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if v, ok := seatEnvValue(env, name); ok {
			t.Errorf("%s=%q on a no-token host", name, v)
		}
	}
	if rec := j.record(t); rec.SeatLogin != "stored" || resp.SeatLogin != "stored" {
		t.Errorf("record %q response %q", rec.SeatLogin, resp.SeatLogin)
	}
}

func TestSeatJourney_TheSeatTokenFileFlagCrossesTheEngineAndLeavesTheHolderAlone(t *testing.T) {
	home := seatHome(t)
	param := filepath.Join(t.TempDir(), "tok")
	if err := os.WriteFile(param, []byte(seatCanaryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	j := newSeatJourney(t)
	resp, err := j.launch(t, func(r *core.BridgeRequest) { r.SeatTokenFile = param })
	if err != nil {
		t.Fatal(err)
	}
	if tok, _ := seatEnvValue(j.callEnv(t), "CLAUDE_CODE_OAUTH_TOKEN"); tok != seatCanaryToken || resp.SeatLogin != "isolated" {
		t.Errorf("the parameter token did not reach the seat: login %q", resp.SeatLogin)
	}
	if _, err := os.Stat(paths.SeatTokenPath(home)); err == nil {
		t.Error("a parameter seat created the holder")
	}
	assertNoCanary(t, j)
}

func TestSeatJourney_ABadHolderEndsTheLaunchAsSeatTokenRefusedWithNoStoredFallback(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, " ", 0o600)
	j := newSeatJourney(t)
	resp, err := j.launch(t, nil)
	if err == nil || resp.ExitCode != ExitSeatTokenRefused || len(j.runner.calls) != 0 {
		t.Fatalf("exit=%d err=%v runs=%d", resp.ExitCode, err, len(j.runner.calls))
	}
	if resp.CauseCode != "seat_token_refused" || !strings.Contains(err.Error(), "is empty, so no claude seat can start") {
		t.Errorf("cause %q err %v", resp.CauseCode, err)
	}
}

func assertNoCanary(t *testing.T, j seatJourney) {
	t.Helper()
	for _, e := range *j.events {
		if strings.Contains(e.Reason, seatCanaryToken) {
			t.Errorf("signal %s holds the canary", e.Code)
		}
		for k, v := range e.Fields {
			if strings.Contains(v, seatCanaryToken) {
				t.Errorf("signal %s field %s holds the canary", e.Code, k)
			}
		}
	}
	for _, call := range j.runner.calls {
		if strings.Contains(strings.Join(call.args, "\x00")+call.name, seatCanaryToken) {
			t.Errorf("argv holds the canary: %v", call.args)
		}
	}
	sawSandboxProfile := false
	_ = filepath.WalkDir(j.ws, func(p string, e os.DirEntry, _ error) error {
		if e == nil || e.IsDir() {
			return nil
		}
		body, _ := os.ReadFile(p)
		if strings.Contains(string(body), seatCanaryToken) {
			t.Errorf("%s holds the canary", p)
		}
		sawSandboxProfile = sawSandboxProfile || strings.HasSuffix(p, ".sb")
		return nil
	})
	if !sawSandboxProfile {
		t.Error("no .sb file was written, so the scan did not cover the sandbox profile")
	}
}
