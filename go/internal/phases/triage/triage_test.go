// Tests for the triage phase. Drives the phase with a fake core.Bridge
// that captures the BridgeRequest and writes a scripted triage-report.md.
package triage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	writeArtifact string
	gotReq        core.BridgeRequest
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fakePromptsFS(body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-triage.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-triage\n---\n" + body),
		},
	})
}

func TestRun_HappyPath_PASSWithTopN(t *testing.T) {
	ws := t.TempDir()
	body := `# Triage Report

## top_n
- id: rate-limit
  priority: high
- id: redact-logs
  priority: medium

## deferred
- id: refactor-mid

## dropped
- id: obsolete-task
`
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.10}}
	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 80*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("triage body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       8,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.Phase != "triage" {
		t.Errorf("Phase=%q, want triage", resp.Phase)
	}
	if resp.NextPhase != "tdd" {
		t.Errorf("NextPhase=%q, want tdd", resp.NextPhase)
	}
	if resp.DurationMS != 80 {
		t.Errorf("DurationMS=%d, want 80", resp.DurationMS)
	}
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "triage-report.md") {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
	}
	wantProfile := filepath.Join("/tmp/proj", ".evolve", "profiles", "triage.json")
	if fb.gotReq.Profile != wantProfile {
		t.Errorf("Profile=%q, want %q", fb.gotReq.Profile, wantProfile)
	}
}

func TestRun_DisabledByEnv_SKIPPED(t *testing.T) {
	// EVOLVE_TRIAGE_DISABLE=1 short-circuits without calling bridge.
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   t.TempDir(),
		Env:         map[string]string{"EVOLVE_TRIAGE_DISABLE": "1"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED", resp.Verdict)
	}
	if resp.NextPhase != "tdd" {
		t.Errorf("NextPhase=%q, want tdd", resp.NextPhase)
	}
	if fb.gotReq.Cycle != 0 {
		t.Errorf("bridge.Launch should not be called when triage disabled")
	}
}

func TestRun_EmptyTopN_FAIL(t *testing.T) {
	body := `# Triage Report

## top_n
_(empty)_

## deferred
- id: a
`
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_PropagatesAsFAIL(t *testing.T) {
	bridgeErr := errors.New("bridge failed")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v, want bridgeErr", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "triage" {
		t.Errorf("Name=%q, want triage", p.Name())
	}
}

// TestRun_HandlesCarryoverTodos verifies the prompt mentions carryover
// todos when supplied via Context (single-line summary; deep schema
// stays in state.json).
func TestRun_HandlesCarryoverTodos(t *testing.T) {
	fb := &fakeBridge{writeArtifact: "## top_n\n- id: x\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"carryover_summary": "2 carryovers: rate-limit (h), redact-logs (m)"},
	})
	if !strings.Contains(fb.gotReq.Prompt, "carryover_summary:") {
		t.Errorf("Prompt missing carryover_summary; got %q", fb.gotReq.Prompt)
	}
}

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeTriageProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// A non-empty report that never declares a "## top_n" section heading must
// FAIL — there is no candidate task list to advance to TDD with. This pins the
// "heading absent entirely" branch (distinct from "## top_n present but empty").
func TestRun_NoTopNHeading_FAIL(t *testing.T) {
	body := `# Triage Report

## deferred
- id: later-task

## dropped
- id: obsolete
`
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no ## top_n heading at all)", resp.Verdict)
	}
}

// The registry init() must publish a "triage" factory that builds a runnable
// PhaseRunner with the production defaults wired (exercises the init closure).
func TestRegistry_TriageFactory_BuildsRunner(t *testing.T) {
	factory, ok := registry.For(string(core.PhaseTriage))
	if !ok {
		t.Fatal(`registry.For("triage") returned ok=false; init() did not register`)
	}
	runner := factory(core.PhaseRequest{ProjectRoot: t.TempDir()})
	if runner == nil {
		t.Fatal("factory returned nil runner")
	}
	if runner.Name() != string(core.PhaseTriage) {
		t.Errorf("Name=%q, want triage", runner.Name())
	}
}
