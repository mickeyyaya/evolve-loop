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

// cycle-138/139 fix: when acs-verdict.json is ABSENT, the audit phase
// generates it (via the injected GenerateVerdict seam → acssuite in prod)
// before reading red_count, so a clean autonomous cycle reaches PASS→ship
// instead of being forced to FAIL on the missing file. The generator
// stand-in here writes a red_count==0 verdict, mimicking a green suite.
func TestRun_MissingACSVerdict_GeneratedThenPASS(t *testing.T) {
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("body"),
		GenerateVerdict: func(req core.PhaseRequest) error {
			genCalls++
			writeACSVerdict(t, req.Workspace, 0) // green suite
			return nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if genCalls != 1 {
		t.Errorf("GenerateVerdict called %d times, want 1 (file was absent)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (verdict generated, red_count=0)", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
	}
}

// A pre-staged acs-verdict.json must be honored as-is: the generator is
// NOT invoked when the file already exists (operator/CI pre-stage path).
func TestRun_ACSVerdictPresent_GeneratorNotCalled(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	genCalls := 0
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { genCalls++; return nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if genCalls != 0 {
		t.Errorf("GenerateVerdict called %d times, want 0 (file pre-staged)", genCalls)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
}

// When the generator runs but produces no verdict file (e.g. zero
// predicates discovered), the missing-file FAIL floor still holds — a
// cycle with nothing to prove must NOT auto-pass.
func TestRun_GeneratorWritesNothing_FAILFloorHolds(t *testing.T) {
	ws := t.TempDir()
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{
		Bridge:          fb,
		Prompts:         fakePromptsFS("body"),
		GenerateVerdict: func(core.PhaseRequest) error { return nil }, // writes no file
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no verdict produced → floor holds)", resp.Verdict)
	}
}

// TestNewDefault_WiresVerdictGenerator pins the cycle-147 fix: the audit phase
// constructed via NewDefault (the single seam now used by BOTH the registry
// init and the loop's runner map in cmd_cycle.go) must wire the REAL
// generateACSVerdict, so a missing acs-verdict.json is auto-generated host-side
// from the on-disk predicate suite — not force-FAILed. This exercises the real
// generateACSVerdict+acssuite path (unlike the fake-generator TestRun_Missing*
// tests) and would have failed against the pre-fix cmd_cycle.go wiring, which
// left GenerateVerdict nil.
func TestNewDefault_WiresVerdictGenerator(t *testing.T) {
	// Predicate root (the cycle worktree): one trivial passing predicate.
	root := t.TempDir()
	predDir := filepath.Join(root, "acs", "cycle-7")
	if err := os.MkdirAll(predDir, 0o755); err != nil {
		t.Fatalf("mkdir preds: %v", err)
	}
	if err := os.WriteFile(filepath.Join(predDir, "001-pass.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write predicate: %v", err)
	}

	// Workspace must be <evolveDir>/runs/cycle-7 so generateACSVerdict's
	// evolveDir = dirname(dirname(workspace)) lands the verdict exactly where
	// Classify reads it (<workspace>/acs-verdict.json).
	evolveDir := t.TempDir()
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := NewDefault(fb, fakePromptsFS("body"))

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 7, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The verdict file must now exist at the canonical path (it did NOT before
	// the wiring fix — that was the cycle-147 forced-FAIL).
	vb, readErr := os.ReadFile(filepath.Join(ws, "acs-verdict.json"))
	if readErr != nil {
		t.Fatalf("acs-verdict.json not generated by NewDefault: %v", readErr)
	}
	var v struct {
		RedCount       int `json:"red_count"`
		PredicateSuite struct {
			ThisCycleCount int `json:"this_cycle_count"`
		} `json:"predicate_suite"`
	}
	if err := json.Unmarshal(vb, &v); err != nil {
		t.Fatalf("verdict parse: %v", err)
	}
	if v.PredicateSuite.ThisCycleCount < 1 {
		t.Errorf("this_cycle_count=%d, want >=1 (the on-disk predicate must be discovered under Worktree)", v.PredicateSuite.ThisCycleCount)
	}
	if v.RedCount != 0 {
		t.Errorf("red_count=%d, want 0 (the only predicate exits 0)", v.RedCount)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "ship" {
		t.Errorf("NextPhase=%q, want ship", resp.NextPhase)
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
