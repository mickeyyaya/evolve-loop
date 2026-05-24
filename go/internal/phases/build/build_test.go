// Tests for the build phase. Verifies cost-threshold wiring and the
// "## Files Modified" verdict trigger.
package build

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
		"agents/evolve-builder.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-builder\n---\n" + body),
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

func TestRun_HappyPath_PASSWithFilesModified(t *testing.T) {
	ws := t.TempDir()
	body := `# Build Report

## Files Modified
- pkg/login/rate_limit.go (NEW, 42 lines)
- pkg/login/handler.go (modified)

## Verdict
**GREEN** — all RED tests now PASS.

## Cost
- USD: 0.85
`
	fb := &fakeBridge{
		writeArtifact: body,
		resp:          core.BridgeResponse{CostUSD: 0.85},
	}
	clock := fixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Builder body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       10,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "audit" {
		t.Errorf("NextPhase=%q, want audit", resp.NextPhase)
	}
	if resp.CostUSD != 0.85 {
		t.Errorf("CostUSD=%v, want 0.85", resp.CostUSD)
	}
	if resp.DurationMS != 200 {
		t.Errorf("DurationMS=%d, want 200", resp.DurationMS)
	}
	wantArtifact := filepath.Join(ws, "build-report.md")
	if fb.gotReq.ArtifactPath != wantArtifact {
		t.Errorf("ArtifactPath=%q, want %q", fb.gotReq.ArtifactPath, wantArtifact)
	}
}

func TestRun_NoFilesModified_FAIL(t *testing.T) {
	body := "# Build Report\n\n## Verdict\nGREEN — no work needed.\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_CostExceedsThreshold_Advisory_PASSWithDiagnostic(t *testing.T) {
	// Advisory mode (EVOLVE_BUILDER_COST_GUARD_STRICT not set): emit a
	// diagnostic but still PASS so the cycle continues. Audit sees the
	// diagnostic and can react.
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{
		writeArtifact: body,
		resp:          core.BridgeResponse{CostUSD: 3.50},
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILDER_COST_THRESHOLD": "2.00"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (advisory)", resp.Verdict)
	}
	foundCostDiag := false
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "cost") && strings.Contains(d.Message, "3.5") {
			foundCostDiag = true
			break
		}
	}
	if !foundCostDiag {
		t.Errorf("expected cost-overrun diagnostic; got %+v", resp.Diagnostics)
	}
}

func TestRun_CostExceedsThreshold_Strict_FAIL(t *testing.T) {
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{
		writeArtifact: body,
		resp:          core.BridgeResponse{CostUSD: 3.50},
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{
			"EVOLVE_BUILDER_COST_THRESHOLD":    "2.00",
			"EVOLVE_BUILDER_COST_GUARD_STRICT": "1",
		},
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (strict cost guard)", resp.Verdict)
	}
}

func TestRun_CostBelowThreshold_NoDiagnostic(t *testing.T) {
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{
		writeArtifact: body,
		resp:          core.BridgeResponse{CostUSD: 1.50},
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILDER_COST_THRESHOLD": "2.00"},
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "cost") {
			t.Errorf("unexpected cost diagnostic at cost below threshold: %v", d)
		}
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

// --- v12.1 Capability 1: plan-mode + extra-flags wiring tests ---

// writeProfile materializes a build profile JSON under
// <projectRoot>/.evolve/profiles/build.json so resolveExtraFlags can
// read it via the real loader. Returns the projectRoot.
func writeProfile(t *testing.T, contents string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return root
}

// TestRun_PopulatesExtraFlagsFromProfile — profile carries extra_flags
// and permission_mode; both must surface in the BridgeRequest the
// phase hands to the bridge.
func TestRun_PopulatesExtraFlagsFromProfile(t *testing.T) {
	root := writeProfile(t, `{
		"agent": "build",
		"extra_flags": ["--require-full"],
		"permission_mode": "acceptEdits"
	}`)
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	for _, want := range []string{"--require-full", "--permission-mode", "acceptEdits"} {
		if !strings.Contains(got, want) {
			t.Errorf("ExtraFlags missing %q; got %v", want, fb.gotReq.ExtraFlags)
		}
	}
}

// TestRun_EnvOverridesProfilePermissionMode — EVOLVE_BUILD_PERMISSION_MODE
// in req.Env beats the profile's permission_mode. The plan calls out
// this precedence explicitly (Capability 1 design).
func TestRun_EnvOverridesProfilePermissionMode(t *testing.T) {
	root := writeProfile(t, `{
		"agent": "build",
		"permission_mode": "acceptEdits"
	}`)
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILD_PERMISSION_MODE": "plan"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	if !strings.Contains(got, "--permission-mode plan") {
		t.Errorf("env override not honored; got %v", fb.gotReq.ExtraFlags)
	}
	if strings.Contains(got, "acceptEdits") {
		t.Errorf("profile value should be overridden, not appended; got %v", fb.gotReq.ExtraFlags)
	}
}

// TestRun_ProfilePermissionModeOnly — env unset, profile sets mode.
// Profile value wins.
func TestRun_ProfilePermissionModeOnly(t *testing.T) {
	root := writeProfile(t, `{
		"agent": "build",
		"permission_mode": "default"
	}`)
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: root, Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	if !strings.Contains(got, "--permission-mode default") {
		t.Errorf("profile permission_mode not propagated; got %v", fb.gotReq.ExtraFlags)
	}
}

// TestRun_NoProfile_EmptyExtraFlags — missing profile is non-fatal;
// ExtraFlags stays nil/empty. Regression guard against breaking the
// build phase on fresh projects with no .evolve/profiles/.
func TestRun_NoProfile_EmptyExtraFlags(t *testing.T) {
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fb.gotReq.ExtraFlags) != 0 {
		t.Errorf("ExtraFlags should be empty when no profile + no env; got %v", fb.gotReq.ExtraFlags)
	}
}

// TestRun_NoProfile_EnvAloneStillWires — even without a profile, the
// env override produces a --permission-mode flag.
func TestRun_NoProfile_EnvAloneStillWires(t *testing.T) {
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: t.TempDir(), Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILD_PERMISSION_MODE": "plan"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.Join(fb.gotReq.ExtraFlags, " ")
	if !strings.Contains(got, "--permission-mode plan") {
		t.Errorf("env-only permission mode not propagated; got %v", fb.gotReq.ExtraFlags)
	}
}

// TestResolveExtraFlags_DirectUnit — direct table-driven test of the
// helper to lock down the precedence rules and the empty-input branch.
func TestResolveExtraFlags_DirectUnit(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T) string // returns profileDir
		envMode  string
		wantFlag bool   // whether --permission-mode appears
		wantMode string // expected mode value when wantFlag=true
	}{
		{
			name:     "no-profile-no-env-empty",
			setup:    func(t *testing.T) string { return t.TempDir() },
			envMode:  "",
			wantFlag: false,
		},
		{
			name: "profile-mode-only",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				_ = os.WriteFile(filepath.Join(d, "build.json"), []byte(`{"permission_mode":"acceptEdits"}`), 0o644)
				return d
			},
			envMode:  "",
			wantFlag: true,
			wantMode: "acceptEdits",
		},
		{
			name:     "env-mode-only",
			setup:    func(t *testing.T) string { return t.TempDir() },
			envMode:  "plan",
			wantFlag: true,
			wantMode: "plan",
		},
		{
			name: "env-beats-profile",
			setup: func(t *testing.T) string {
				d := t.TempDir()
				_ = os.WriteFile(filepath.Join(d, "build.json"), []byte(`{"permission_mode":"acceptEdits"}`), 0o644)
				return d
			},
			envMode:  "plan",
			wantFlag: true,
			wantMode: "plan",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := c.setup(t)
			got := resolveExtraFlags(dir, "build", c.envMode)
			joined := strings.Join(got, " ")
			if c.wantFlag {
				if !strings.Contains(joined, "--permission-mode "+c.wantMode) {
					t.Errorf("want flag --permission-mode %s; got %v", c.wantMode, got)
				}
			} else if strings.Contains(joined, "--permission-mode") {
				t.Errorf("did not want --permission-mode flag; got %v", got)
			}
		})
	}
}

// TestResolveExtraFlags_ProfileFlagsAlwaysIncluded — extra_flags from
// the profile pass through regardless of permission_mode setting.
func TestResolveExtraFlags_ProfileFlagsAlwaysIncluded(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "build.json"),
		[]byte(`{"extra_flags":["--require-full","--print"]}`), 0o644)
	got := resolveExtraFlags(d, "build", "")
	for _, want := range []string{"--require-full", "--print"} {
		found := false
		for _, f := range got {
			if f == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ExtraFlags missing %q from profile; got %v", want, got)
		}
	}
}

// TestResolveExtraFlags_MalformedProfileNonFatal — broken JSON in the
// profile must not crash; helper returns env-driven flags (if any) or
// nil. Protects fresh-cluster bootstrap from a corrupted profile file.
func TestResolveExtraFlags_MalformedProfileNonFatal(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "build.json"), []byte("not-json"), 0o644)
	got := resolveExtraFlags(d, "build", "plan")
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--permission-mode plan") {
		t.Errorf("env mode should still propagate over malformed profile; got %v", got)
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
	if p.Name() != "build" {
		t.Errorf("Name=%q, want build", p.Name())
	}
}

func TestRun_InvalidThresholdEnv_FallsBackToDefault(t *testing.T) {
	// Malformed threshold parses as 0, but the package default (2.00)
	// kicks in via parseFloatOrDefault.
	body := "# Build Report\n## Files Modified\n- a.go\n"
	fb := &fakeBridge{
		writeArtifact: body,
		resp:          core.BridgeResponse{CostUSD: 1.50},
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{"EVOLVE_BUILDER_COST_THRESHOLD": "not-a-number"},
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (default threshold 2.00, cost 1.50)", resp.Verdict)
	}
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "cost") {
			t.Errorf("unexpected cost diagnostic: %v", d)
		}
	}
}
