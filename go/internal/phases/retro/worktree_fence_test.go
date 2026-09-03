package retro

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// mutatingRetroBridge plays a retro agent that edits the worktree it was told
// to inspect, then writes its report.
type mutatingRetroBridge struct{ fakeBridge }

func (b *mutatingRetroBridge) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.Worktree != "" {
		_ = os.WriteFile(filepath.Join(req.Worktree, "src", "mat.go"), []byte("package src // reverted by retro\n"), 0o644)
		_ = os.WriteFile(filepath.Join(req.Worktree, "src", "zz_probe_test.go"), []byte("package src\n"), 0o644)
	}
	return b.fakeBridge.Launch(ctx, req)
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
	if err := os.WriteFile(filepath.Join(dir, "src", "mat.go"), []byte("package src // builder change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRun_ReadOnlyWorktreeIsFencedAroundRetrosOwnLaunch — retro calls the
// bridge itself (not through phases/runner), so it must hold the fence
// itself: the tree it hands to the retry envelope is the tree it was given,
// and the write is reported on its response.
func TestRun_ReadOnlyWorktreeIsFencedAroundRetrosOwnLaunch(t *testing.T) {
	dir := fenceRepo(t)
	fb := &mutatingRetroBridge{fakeBridge{writeArtifact: "# Retrospective\n\n## Verdict\nFAIL\n"}}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 7, ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: dir, WorktreeReadOnly: true,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.launches == 0 {
		t.Fatal("bridge was not launched")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go"))
	if string(got) != "package src // builder change\n" {
		t.Errorf("retro's edit not undone: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "zz_probe_test.go")); !os.IsNotExist(err) {
		t.Error("retro's probe file must be removed")
	}
	found := false
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "worktree fence: read-only phase retro wrote 2 path(s)") {
			found = true
		}
	}
	if !found {
		t.Errorf("the write must be reported on retro's response, diags=%+v", resp.Diagnostics)
	}
}
