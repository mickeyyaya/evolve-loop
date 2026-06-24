package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRun_StdoutFilter_InvokedOnSuccess(t *testing.T) {
	var called struct {
		count     int
		workspace string
		phase     string
	}
	hooks := &fakeHooks{
		phase: "build", agent: "evolve-builder", model: "sonnet",
		prompt: "p", verdict: core.VerdictPASS, nextPhase: "audit",
	}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{writeArtifact: "ok\n"},
		Prompts: fakePromptsFS("evolve-builder", "body"),
		NowFn:   fixtures.FixedClock(time.Unix(1, 0), time.Millisecond),
		StdoutFilter: func(workspace, phase string) error {
			called.count++
			called.workspace = workspace
			called.phase = phase
			return nil
		},
	})
	ws := t.TempDir()
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: ws,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called.count != 1 {
		t.Fatalf("StdoutFilter call count=%d, want 1", called.count)
	}
	if called.workspace != ws {
		t.Errorf("workspace=%q, want %q", called.workspace, ws)
	}
	if called.phase != "build" {
		t.Errorf("phase=%q, want build", called.phase)
	}
}

func TestRun_StdoutFilter_ErrorDoesNotBlockPhase(t *testing.T) {
	hooks := &fakeHooks{
		phase: "scout", agent: "evolve-scout", model: "sonnet",
		prompt: "p", verdict: core.VerdictPASS, nextPhase: "triage",
	}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{writeArtifact: "ok\n"},
		Prompts: fakePromptsFS("evolve-scout", "body"),
		NowFn:   fixtures.FixedClock(time.Unix(1, 0), time.Millisecond),
		StdoutFilter: func(workspace, phase string) error {
			return errors.New("synthetic filter blowup")
		},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run must not fail on filter error: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (filter error is non-blocking)", resp.Verdict)
	}
}

func TestRun_StdoutFilter_OffEnvSkipsFilter(t *testing.T) {
	var called int
	hooks := &fakeHooks{
		phase: "scout", agent: "evolve-scout", model: "sonnet",
		prompt: "p", verdict: core.VerdictPASS, nextPhase: "triage",
	}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{writeArtifact: "ok\n"},
		Prompts: fakePromptsFS("evolve-scout", "body"),
		NowFn:   fixtures.FixedClock(time.Unix(1, 0), time.Millisecond),
		StdoutFilter: func(workspace, phase string) error {
			called++
			return nil
		},
		DisableStdoutFilter: true,
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if called != 0 {
		t.Fatalf("StdoutFilter must NOT be called when DisableStdoutFilter=true; called=%d", called)
	}
}

// TestRun_StdoutFilter_E2E_WritesCompanionFile exercises the real
// logfilter.Process end-to-end through the runner: bridge writes a
// stream-json stdout to disk, runner triggers the default-on filter,
// and a clean.txt companion appears.
func TestRun_StdoutFilter_E2E_WritesCompanionFile(t *testing.T) {
	ws := t.TempDir()
	raw := `{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"signal preserved"}]}}` + "\n" +
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"noise"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(raw), 0o644); err != nil {
		t.Fatalf("seed raw: %v", err)
	}

	hooks := &fakeHooks{
		phase: "build", agent: "evolve-builder", model: "sonnet",
		prompt: "p", verdict: core.VerdictPASS, nextPhase: "audit",
	}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{writeArtifact: "ok\n"},
		Prompts: fakePromptsFS("evolve-builder", "body"),
		NowFn:   fixtures.FixedClock(time.Unix(1, 0), time.Millisecond),
		// StdoutFilter unset → defaults to real logfilter.Process
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: ws,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	clean, err := os.ReadFile(filepath.Join(ws, "build-stdout.clean.txt"))
	if err != nil {
		t.Fatalf("expected build-stdout.clean.txt to exist: %v", err)
	}
	got := string(clean)
	if !strings.Contains(got, "signal preserved") {
		t.Errorf("clean must contain assistant text, got %q", got)
	}
	if strings.Contains(got, "stream_event") {
		t.Errorf("clean must NOT contain stream_event, got %q", got)
	}
}
