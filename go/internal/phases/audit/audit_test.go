// Tests for the audit phase. Audit is the EGPS gate: PASS requires
// BOTH a parseable PASS verdict in audit-report.md AND red_count == 0
// in acs-verdict.json.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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
		"agents/evolve-auditor.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-auditor\n---\n" + body),
		},
	})
}

func fixedClock(t time.Time, dur time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		defer func() { calls++ }()
		if calls == 0 {
			return t
		}
		return t.Add(dur)
	}
}

// writeACSVerdict writes a verdict.json to ws/acs-verdict.json with the
// given red_count.
func writeACSVerdict(t *testing.T, ws string, redCount int) {
	t.Helper()
	v := map[string]any{
		"cycle":      42,
		"red_count":  redCount,
		"total":      10,
		"predicates": []any{},
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

func TestRun_HappyPath_PASS(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n\nNo defects found.\n"
	fb := &fakeBridge{writeArtifact: body, resp: core.BridgeResponse{CostUSD: 0.30}}
	clock := fixedClock(time.Unix(1_700_000_000, 0), 60*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Auditor body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 42, ProjectRoot: "/tmp/proj", Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
	}
	if resp.DurationMS != 60 {
		t.Errorf("DurationMS=%d, want 60", resp.DurationMS)
	}
	wantArtifact := filepath.Join(ws, "audit-report.md")
	if fb.gotReq.ArtifactPath != wantArtifact {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
	}
}

func TestRun_AuditPASSButRedCountNonZero_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 3)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (EGPS red_count=3)", resp.Verdict)
	}
	gotEGPSDiag := false
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "red_count") {
			gotEGPSDiag = true
		}
	}
	if !gotEGPSDiag {
		t.Errorf("missing red_count diagnostic; got %+v", resp.Diagnostics)
	}
}

func TestRun_AuditFAIL_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**FAIL**\n\nDefect: missing auth check.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (audit said FAIL)", resp.Verdict)
	}
}

func TestRun_AuditWARN_WARN(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**WARN**\n\nMinor cleanup recommended.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictWARN {
		t.Errorf("Verdict=%q, want WARN", resp.Verdict)
	}
}

func TestRun_StrictAuditMode_WARNBecomesFAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**WARN**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
		Env: map[string]string{"EVOLVE_STRICT_AUDIT": "1"},
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (strict-audit promotes WARN→FAIL)", resp.Verdict)
	}
}

func TestRun_NoVerdictHeading_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\nSome prose without a verdict heading.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingACSVerdict_FAIL(t *testing.T) {
	// No acs-verdict.json on disk = cycle cannot prove EGPS gate → FAIL.
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no ACS verdict file)", resp.Verdict)
	}
}

func TestRun_ACSVerdictMalformed_FAIL(t *testing.T) {
	ws := t.TempDir()
	_ = os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte("not json"), 0o644)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (ACS verdict unparseable)", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	fb := &fakeBridge{writeArtifact: ""}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_BridgeError_FAIL(t *testing.T) {
	bridgeErr := errors.New("bridge fail")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if !errors.Is(err, bridgeErr) {
		t.Errorf("err=%v", err)
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
	if p.Name() != "audit" {
		t.Errorf("Name=%q, want audit", p.Name())
	}
}

// --- v12.1 Capability 1: phaseflags wiring tests ---

func writeAuditProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auditor.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}
