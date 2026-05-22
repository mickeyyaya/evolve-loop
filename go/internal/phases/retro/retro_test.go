// Tests for the retro phase. Retro is a conditional phase: it runs
// only when the previous verdict is FAIL or WARN; otherwise SKIPPED
// without calling the bridge.
package retro

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
	writeLesson   string
	gotReq        core.BridgeRequest
}

func (f *fakeBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	if f.writeArtifact != "" && req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(f.writeArtifact), 0o644)
		f.resp.Stdout = f.writeArtifact
	}
	if f.writeLesson != "" {
		_ = os.WriteFile(filepath.Join(req.Workspace, "failure-lesson.yaml"), []byte(f.writeLesson), 0o644)
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
	clock := fixedClock(time.Unix(1_700_000_000, 0), 90*time.Millisecond)
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
	if fb.gotReq.ArtifactPath != filepath.Join(ws, "retrospective.md") {
		t.Errorf("ArtifactPath=%q", fb.gotReq.ArtifactPath)
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

func TestRun_BridgeError_FAIL(t *testing.T) {
	bridgeErr := errors.New("bridge fail")
	fb := &fakeBridge{err: bridgeErr}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
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
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil || !strings.Contains(err.Error(), "bridge required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_MissingPrompts_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil || !strings.Contains(err.Error(), "prompts loader required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRun_AgentLoadFails_ReturnsError(t *testing.T) {
	phase := New(Config{Bridge: &fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err == nil {
		t.Fatal("err=nil")
	}
}

func TestRun_DisabledByEnv_SKIPPED(t *testing.T) {
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Env: map[string]string{
			"EVOLVE_DISABLE_AUTO_RETROSPECTIVE": "1",
		},
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("Verdict=%q, want SKIPPED (auto-retro disabled)", resp.Verdict)
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
