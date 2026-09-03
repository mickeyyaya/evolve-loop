package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// mutatingBridge plays the cycle-1603 auditor: it rewrites a material file in
// place and drops a probe test into the package, then writes its report.
type mutatingBridge struct{ launches int }

func (b *mutatingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.launches++
	if req.Worktree != "" {
		_ = os.WriteFile(filepath.Join(req.Worktree, "src", "mat.go"), []byte("package src // MUTATED by a probe\n"), 0o644)
		_ = os.WriteFile(filepath.Join(req.Worktree, "src", "zz_probe_test.go"), []byte("package src // probe\n"), 0o644)
	}
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("ok"), 0o644)
	}
	return core.BridgeResponse{Stdout: "ok"}, nil
}
func (b *mutatingBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func fenceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "mat.go"), []byte("package src // base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	// The builder's pending (uncommitted) change — what the audit must judge.
	if err := os.WriteFile(filepath.Join(dir, "src", "mat.go"), []byte("package src // builder change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runFenced(t *testing.T, phase string, readOnly bool) (dir string, resp core.PhaseResponse) {
	dir, resp, _ = runFencedObserving(t, phase, readOnly)
	return dir, resp
}

// runFencedObserving also returns what src/mat.go held when Classify ran —
// the audit's explanation binding runs inside Classify, so the restore must
// precede it, not merely happen.
func runFencedObserving(t *testing.T, phase string, readOnly bool) (dir string, resp core.PhaseResponse, atClassify string) {
	t.Helper()
	dir = fenceRepo(t)
	hooks := &fakeHooks{phase: phase, agent: "evolve-" + phase, model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	hooks.onClassify = func(req core.PhaseRequest) {
		b, _ := os.ReadFile(filepath.Join(req.Worktree, "src", "mat.go"))
		atClassify = string(b)
	}
	r := New(Options{Hooks: hooks, Bridge: &mutatingBridge{}, Prompts: fakePromptsFS("evolve-"+phase, "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: dir, WorktreeReadOnly: readOnly}
	resp, err := r.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return dir, resp, atClassify
}

func fenceDiagnostic(resp core.PhaseResponse) string {
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "worktree fence:") {
			return d.Message
		}
	}
	return ""
}

// TestRun_ReadOnlyPhaseWorktreeIsRestoredAndReported — the auditor's probes
// are undone before the response is classified, and the write is named.
func TestRun_ReadOnlyPhaseWorktreeIsRestoredAndReported(t *testing.T) {
	t.Parallel()
	dir, resp, atClassify := runFencedObserving(t, "audit", true)
	if atClassify != "package src // builder change\n" {
		t.Errorf("Classify must run on the RESTORED tree (the explanation binding lives there), saw %q", atClassify)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go"))
	if string(got) != "package src // builder change\n" {
		t.Errorf("material file not restored to the builder's content: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "zz_probe_test.go")); !os.IsNotExist(err) {
		t.Error("the probe file must be removed")
	}
	msg := fenceDiagnostic(resp)
	if !strings.Contains(msg, "read-only phase audit wrote 2 path(s)") || !strings.Contains(msg, "src/mat.go") || !strings.Contains(msg, "src/zz_probe_test.go") {
		t.Errorf("the write must be reported on the response with the restored paths, got %q (diags=%+v)", msg, resp.Diagnostics)
	}
	// The fence reports; it never decides. Same fixture unfenced ⇒ same verdict.
	if _, unfenced := runFenced(t, "audit", false); unfenced.Verdict != resp.Verdict {
		t.Errorf("the fence changed the verdict: fenced=%s unfenced=%s", resp.Verdict, unfenced.Verdict)
	}
}

// TestRun_SourceWriterIsNotFenced — build/tdd keep their writes and get no
// fence diagnostic (byte-identical to before the fence).
func TestRun_SourceWriterIsNotFenced(t *testing.T) {
	t.Parallel()
	dir, resp := runFenced(t, "build", false)
	got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go"))
	if string(got) != "package src // MUTATED by a probe\n" {
		t.Errorf("a source writer's writes must be kept: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "zz_probe_test.go")); err != nil {
		t.Error("a source writer's new file must be kept")
	}
	if msg := fenceDiagnostic(resp); msg != "" {
		t.Errorf("no fence diagnostic for a source writer, got %q", msg)
	}
}

// TestRun_FenceUnavailableWarnsAndProceeds — a worktree that is not a
// repository cannot be fenced: the phase runs, and the response says so.
func TestRun_FenceUnavailableWarnsAndProceeds(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "audit", agent: "evolve-audit", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &mutatingBridge{}, Prompts: fakePromptsFS("evolve-audit", "body")})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir(), WorktreeReadOnly: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if msg := fenceDiagnostic(resp); !strings.Contains(msg, "snapshot unavailable") {
		t.Errorf("an untakeable fence must be reported, got %q", msg)
	}
}
