// Tests for the retro phase. Retro is a conditional phase: it runs
// only when the previous verdict is FAIL or WARN; otherwise SKIPPED
// without calling the bridge.
package retro

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type fakeBridge struct {
	resp          core.BridgeResponse
	err           error
	errs          []error
	writeArtifact string
	writeLesson   string
	gotReq        core.BridgeRequest
	launches      int
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	call := f.launches
	f.launches++
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	if f.writeLesson != "" {
		_ = os.WriteFile(filepath.Join(req.Workspace, "failure-lesson.yaml"), []byte(f.writeLesson), 0o644)
	}
	if call < len(f.errs) {
		return f.resp, f.errs[call]
	}
	return f.resp, f.err
}

func (f *fakeBridge) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fakePromptsFS(body string) *prompts.Loader {
	return prompts.NewFromFS(fstest.MapFS{
		"agents/evolve-retrospective.md": &fstest.MapFile{
			Data: []byte("---\nname: evolve-retrospective\n---\n" + body),
		},
	})
}

func TestRun_PreviousPASS_SKIPPEDWithoutBridgeCall(t *testing.T) {
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictPASS},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED for previous=PASS", resp.Verdict)
	}
	if resp.NextPhase != "end" {
		t.Errorf("NextPhase=%q, want end", resp.NextPhase)
	}
	if fb.gotReq.Cycle != 0 {
		t.Errorf("bridge.Launch called when previous=PASS; should short-circuit")
	}
}

func TestRun_PreviousFAIL_PASSWithLesson(t *testing.T) {
	ws := t.TempDir()
	body := "# Retrospective\n\n## Root Cause\nMissing rate limit.\n\n## Lessons\nApply rate limiter pattern.\n"
	lesson := "id: rate-limit-missing\ntags: [auth, security]\nlesson: install rate limiter\n"
	fb := &fakeBridge{writeArtifact: body, writeLesson: lesson, resp: core.BridgeResponse{CostUSD: 0.15}}
	clock := fixtures.FixedClock(time.Unix(1_700_000_000, 0), 90*time.Millisecond)
	phase := New(Config{
		Bridge:  fb,
		Prompts: fakePromptsFS("# Retro body"),
		NowFn:   clock,
	})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 5, ProjectRoot: "/tmp/proj", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "end" {
		t.Errorf("NextPhase=%q, want end", resp.NextPhase)
	}
	if resp.DurationMS != 90 {
		t.Errorf("DurationMS=%d, want 90", resp.DurationMS)
	}
	// cycle-187 AC-5/AC-6: retro must poll the file the evolve-retrospective
	// agent actually writes — "retrospective-report.md" — not the stale
	// "retrospective.md" the runner used before. The mismatch made the bridge
	// time out on every retro invocation (Scout Gap B).
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "retrospective-report.md") {
		t.Errorf("ArtifactPath=%q, want retrospective-report.md (agent output path)", fb.gotReq.ArtifactPath)
	}
	wantProfile := filepath.Join("/tmp/proj", ".evolve", "profiles", "retrospective.json")
	if fb.gotReq.Profile != wantProfile {
		t.Errorf("Profile=%q, want %q", fb.gotReq.Profile, wantProfile)
	}
}

func TestRun_PreviousWARN_PASSWithLesson(t *testing.T) {
	ws := t.TempDir()
	body := "# Retrospective\n## Root Cause\nminor.\n## Lessons\nfollow-up.\n"
	lesson := "id: minor-issue\n"
	fb := &fakeBridge{writeArtifact: body, writeLesson: lesson}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 5, ProjectRoot: "/p", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictWARN},
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
}

func TestRun_NoLessonWritten_FAIL(t *testing.T) {
	ws := t.TempDir()
	body := "# Retrospective\n## Root Cause\nx\n## Lessons\nfollow-up\n"
	// fakeBridge writes the report but no failure-lesson*.yaml.
	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (no failure-lesson YAML)", resp.Verdict)
	}
}

func TestRun_EmptyArtifact_FAIL(t *testing.T) {
	fb := &fakeBridge{writeArtifact: "", writeLesson: "id: x"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

// GAP 9 (self-healing): a retro BRIDGE failure must NOT propagate a fatal error.
// Retro is the failure-analysis phase on the audit-FAIL path; a RunCycle error
// stops the whole batch (the cause of the runs 154-162 aborts). If retro's own
// bridge dies, it returns a FAIL verdict with NIL error so the orchestrator routes
// through decideAfterRetro (failure-adapter: retry/block/proceed) instead of
// hard-aborting the cycle AND the batch. The bridge error is preserved as an error
// diagnostic for forensics. (A failure in the failure-handler must never be fatal.)
func TestRun_BridgeError_FAIL(t *testing.T) {
	bridgeErr := errors.New("bridge boot timeout")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("retro bridge failure must NOT return a fatal error (it would abort the whole batch); got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL so the orchestrator routes via decideAfterRetro", resp.Verdict)
	}
	if len(resp.Diagnostics) == 0 || resp.Diagnostics[0].Severity != "error" {
		t.Fatalf("bridge error must be preserved as an error diagnostic for forensics; got %+v", resp.Diagnostics)
	}
}

func TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce(t *testing.T) {
	ws := t.TempDir()
	fb := &fakeBridge{
		errs:          []error{submitWedgedTimeoutErr(), nil},
		writeArtifact: "# Retrospective\n## Root Cause\ndelivery stalled\n## Lessons\nrelaunch verified\n",
		writeLesson:   "id: retry-delivery-failure\n",
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.launches != 2 {
		t.Fatalf("Launch calls=%d, want 2: a verified submit_wedged delivery failure must be relaunched once", fb.launches)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS after the relaunch succeeds", resp.Verdict)
	}
}

func TestRun_GenericArtifactTimeout_DoesNotRelaunch(t *testing.T) {
	fb := &fakeBridge{err: genericSilenceTimeoutErr()}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.launches != 1 {
		t.Fatalf("Launch calls=%d, want 1: a generic silence timeout has no verified delivery failure and must not relaunch", fb.launches)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL after the generic timeout", resp.Verdict)
	}
}

func submitWedgedTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro waited=0s interval=300s transient=false reason=%q: %w",
		"prompt submit_wedged (resends=3)", core.ErrArtifactTimeout)
}

func genericSilenceTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro waited=1800s interval=900s transient=false reason=%q: %w",
		"no output during the last 900s interval — stalled; pause for investigation", core.ErrArtifactTimeout)
}

func TestRun_MissingBridge_ReturnsError(t *testing.T) {
	phase := New(Config{Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:   1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:   1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:   1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestRun_RetiredDisableEnvIgnored(t *testing.T) {
	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: retired-env-ignored\nlesson: retro still runs\n",
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{
			"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": "1",
		},
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (retired env ignored)", resp.Verdict)
	}
	if fb.gotReq.Cycle != 1 {
		t.Error("bridge.Launch was not called for FAIL verdict with retired env set")
	}
}

func TestRun_AcceptsAnyFailureLessonFilename(t *testing.T) {
	ws := t.TempDir()
	body := "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n"
	fb := &fakeBridge{writeArtifact: body}
	// Pre-write a lesson file with a hash-suffix name pattern (real
	// fixtures use failure-lesson-{shortsha}.yaml).
	_ = os.WriteFile(filepath.Join(ws, "failure-lesson-abc123.yaml"), []byte("id: x\n"), 0o644)
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: ws,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (lesson file with shortsha suffix should count)", resp.Verdict)
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "retro" {
		t.Errorf("Name=%q, want retro", p.Name())
	}
}

// TestHasFailureLesson_NonexistentWorkspace_False exercises the
// os.ReadDir error path: when the workspace doesn't exist, the helper
// returns false (and the run path treats that as no-lesson-written).
func TestHasFailureLesson_NonexistentWorkspace_False(t *testing.T) {
	got := hasFailureLesson("/path/that/does/not/exist/at/all")
	if got {
		t.Errorf("hasFailureLesson on missing dir = true, want false")
	}
}

// TestHasFailureLesson_IgnoresDirectoriesAndOtherFiles verifies the
// helper skips directories and non-matching filenames.
func TestHasFailureLesson_IgnoresDirectoriesAndOtherFiles(t *testing.T) {
	ws := t.TempDir()
	_ = os.MkdirAll(filepath.Join(ws, "failure-lesson-subdir"), 0o755)
	_ = os.WriteFile(filepath.Join(ws, "lesson.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(ws, "failure-lesson"), []byte("x"), 0o644) // no .yaml
	if hasFailureLesson(ws) {
		t.Errorf("returned true with no matching .yaml; want false")
	}
}

// --- retro-model-auto-normalization (Bug B) -------------------------------
//
// Retro is a hand-rolled runner: it never passes through BaseRunner, so it
// never reaches the single dispatch resolver that expands the "auto" model
// sentinel (llmroute.Resolve → resolvellm). These tests pin the dispatched
// core.BridgeRequest.Model — the value that actually reaches the CLI — rather
// than resolvellm in isolation.

func writeRetroProfileDoc(t *testing.T, projectRoot, body string) {
	t.Helper()
	dir := filepath.Join(projectRoot, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "retrospective.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write retrospective profile: %v", err)
	}
}

// runRetroForModel drives the real Phase.Run route against a temp project root
// holding the given retrospective profile, and returns the dispatched request.
func runRetroForModel(t *testing.T, cfgModel, profile string) core.BridgeRequest {
	t.Helper()
	// Isolate from any ambient profile roots so the temp project root is the
	// only profile source resolvellm can see.
	t.Setenv("EVOLVE_PLUGIN_ROOT", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", "")

	projectRoot := t.TempDir()
	writeRetroProfileDoc(t, projectRoot, profile)

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: model-normalization\n",
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body"), Model: cfgModel})
	if _, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: projectRoot, Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.launches != 1 {
		t.Fatalf("Launch calls=%d, want 1", fb.launches)
	}
	return fb.gotReq
}

const retroProfileDeep = `{"name":"retrospective","role":"retrospective","cli":"codex-tmux","model_tier_default":"deep"}`

// AC: Retro never sends BridgeRequest.Model == "auto" — the unset/sentinel case
// must resolve to the profile's tier before dispatch.
func TestRun_AutoModel_ResolvedBeforeDispatch(t *testing.T) {
	got := runRetroForModel(t, "", retroProfileDeep)
	if got.Model == "auto" {
		t.Fatalf("BridgeRequest.Model=%q — the unresolved sentinel reached the bridge", got.Model)
	}
	if got.Model != "deep" {
		t.Errorf("BridgeRequest.Model=%q, want %q (retrospective profile model_tier_default)", got.Model, "deep")
	}
}

// AC: an explicitly configured tier survives unchanged (no resolution applied).
func TestRun_ExplicitModel_PassesThroughUnchanged(t *testing.T) {
	got := runRetroForModel(t, "balanced", retroProfileDeep)
	if got.Model != "balanced" {
		t.Errorf("BridgeRequest.Model=%q, want %q — an explicit tier must not be re-resolved", got.Model, "balanced")
	}
}

// AC (edge): a profile with no model_tier_default must still not dispatch the
// sentinel — it resolves to the established default tier.
func TestRun_AutoModel_ProfileWithoutTier_ResolvesToDefaultNotAuto(t *testing.T) {
	got := runRetroForModel(t, "", `{"name":"retrospective","role":"retrospective","cli":"codex-tmux"}`)
	if got.Model == "auto" || got.Model == "" {
		t.Fatalf("BridgeRequest.Model=%q — want a resolved tier, never the sentinel", got.Model)
	}
	if got.Model != "balanced" {
		t.Errorf("BridgeRequest.Model=%q, want %q (resolvellm's established default tier)", got.Model, "balanced")
	}
}

// AC (negative/OOD): the sentinel must never survive resolution even when the
// project root has no retrospective profile at all.
func TestRun_AutoModel_NoProfile_NeverDispatchesSentinel(t *testing.T) {
	t.Setenv("EVOLVE_PLUGIN_ROOT", "")
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	projectRoot := t.TempDir()

	fb := &fakeBridge{
		writeArtifact: "# Retrospective\n## Root Cause\nx\n## Lessons\ny\n",
		writeLesson:   "id: model-normalization\n",
	}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: projectRoot, Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		// Failing loudly instead of dispatching the sentinel is an acceptable
		// resolution of this AC; it must not have launched with "auto".
		if fb.launches > 0 && fb.gotReq.Model == "auto" {
			t.Fatalf("launched with Model=%q before failing", fb.gotReq.Model)
		}
		return
	}
	if fb.gotReq.Model == "auto" {
		t.Fatalf("BridgeRequest.Model=%q — the sentinel reached the bridge with no profile present", fb.gotReq.Model)
	}
}
