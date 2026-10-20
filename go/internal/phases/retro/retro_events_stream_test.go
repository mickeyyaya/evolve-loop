package retro

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseoutputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type streamingBridge struct {
	agents []string
}

func (b *streamingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.agents = append(b.agents, req.Agent)
	report := "# Retrospective\n## Root Cause\nthe gate ran in the lane env\n## Lessons\nscrub it\n"
	_ = os.WriteFile(filepath.Join(req.Workspace, req.Agent+"-stdout.log"), []byte("retro agent: reading the failure digest\nretro agent: wrote the lesson\n"), 0o644)
	_ = os.WriteFile(req.ArtifactPath, []byte(report), 0o644)
	_ = os.WriteFile(filepath.Join(req.Workspace, "failure-lesson.yaml"), []byte("id: gate-env\n"), 0o644)
	return core.BridgeResponse{Stdout: report}, nil
}

func (b *streamingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func workspaceListing(t *testing.T, ws string) map[string]int64 {
	t.Helper()
	entries, err := os.ReadDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	listing := map[string]int64{}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() {
			listing[e.Name()] = info.Size()
		}
	}
	return listing
}

func TestRun_AFailedCyclesRetroLeavesItsEventsStreamForTheSurvey(t *testing.T) {
	isolateFromAmbientProfileRoots(t)
	ws := t.TempDir()
	br := &streamingBridge{}

	resp, err := New(Config{Bridge: br, Prompts: fakePromptsFS("body")}).Run(context.Background(), core.PhaseRequest{
		Cycle: 7, ProjectRoot: t.TempDir(), Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})

	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("Run: verdict=%q err=%v diags=%+v", resp.Verdict, err, resp.Diagnostics)
	}
	row := phaseoutputs.Survey([]string{string(core.PhaseRetro)}, workspaceListing(t, ws), nil).Rows[0]
	if !row.Events.Present || row.Events.Empty {
		t.Fatalf("the retro left no events stream for the survey: %+v (agents launched %v)", row.Events, br.agents)
	}
}

func TestDeliveryRelaunch_ACancelledContextNeverRelaunches(t *testing.T) {
	fb := &fakeBridge{errs: []error{submitWedgedTimeoutErr(), nil}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := relaunchingBridge(fb).Launch(ctx, core.BridgeRequest{CLI: "claude-tmux"})

	if fb.launches != 1 || err == nil {
		t.Fatalf("launches=%d err=%v: a cancelled dispatch keeps its delivery failure and is not relaunched", fb.launches, err)
	}
}

type signallingBridge struct {
	fakeBridge
	center *signalcenter.Center
}

func (b *signallingBridge) Signals() *signalcenter.Center { return b.center }

func TestNew_TheRetroJudgesWithTheSignalsOfItsBridge(t *testing.T) {
	if New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")}).SignalsWired() {
		t.Fatal("a bridge without signals wired some")
	}
	wired := New(Config{Bridge: &signallingBridge{center: signalcenter.New()}, Prompts: fakePromptsFS("body")})
	if !wired.SignalsWired() {
		t.Fatal("the retro dropped the signals of the walking bridge that every other phase judges with")
	}
}

func TestRun_TheRetroDeclaresItsDispositionAndCarryoverAsSecondaryArtifacts(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n", writeLesson: "id: x\n"}

	if _, err := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")}).Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictWARN},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{filepath.Join(ws, "disposition.json"), filepath.Join(ws, "carryover-todos.json")}
	if !reflect.DeepEqual(fb.gotReq.SecondaryArtifacts, want) {
		t.Fatalf("SecondaryArtifacts=%v, want %v", fb.gotReq.SecondaryArtifacts, want)
	}
	if fb.gotReq.Agent != string(core.PhaseRetro) {
		t.Fatalf("Agent=%q, want %q: the bridge names its stdout log after the agent, and phasestream reads <phase>-stdout.log", fb.gotReq.Agent, core.PhaseRetro)
	}
}

type attemptLoggingBridge struct {
	launches int
}

func (b *attemptLoggingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.launches++
	attempt := "attempt " + string(rune('0'+b.launches))
	_ = os.WriteFile(filepath.Join(req.Workspace, req.Agent+"-stdout.log"), []byte(attempt+" stdout\n"), 0o644)
	_ = os.WriteFile(filepath.Join(req.Workspace, req.Agent+"-stderr.log"), []byte(attempt+" stderr\n"), 0o644)
	if b.launches == 1 {
		return core.BridgeResponse{ExitCode: 81}, submitWedgedTimeoutErr()
	}
	return core.BridgeResponse{}, nil
}

func (b *attemptLoggingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func TestDeliveryRelaunch_TheFirstAttemptsCaptureSurvivesTheRelaunch(t *testing.T) {
	ws := t.TempDir()
	br := &attemptLoggingBridge{}

	if _, err := relaunchingBridge(br).Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Agent: "retro", Workspace: ws}); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	if br.launches != 2 {
		t.Fatalf("launches=%d, want 2", br.launches)
	}
	for stream, want := range map[string]string{"stdout": "attempt 1 stdout\n", "stderr": "attempt 1 stderr\n"} {
		if got := readLog(t, filepath.Join(ws, "retro-"+stream+".attempt-1.log")); got != want {
			t.Errorf("attempt-1 %s capture = %q, want %q", stream, got, want)
		}
	}
	if got := readLog(t, filepath.Join(ws, "retro-stdout.log")); got != "attempt 2 stdout\n" {
		t.Errorf("the stdout log phasestream reads = %q, want the final attempt", got)
	}
}

func TestDeliveryRelaunch_ALogThatCannotBeKeptIsReportedAndTheRelaunchStillRuns(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "retro-stdout.attempt-1.log", "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	br := &attemptLoggingBridge{}

	if _, err := relaunchingBridge(br).Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Agent: "retro", Workspace: ws}); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	if br.launches != 2 {
		t.Fatalf("launches=%d, want 2: a log that cannot be kept must not stop the relaunch", br.launches)
	}
}
