package retro

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func retroFailReq(projectRoot, workspace, worktree string, env map[string]string) core.PhaseRequest {
	return core.PhaseRequest{
		Cycle:       1255,
		ProjectRoot: projectRoot,
		Workspace:   workspace,
		Worktree:    worktree,
		Env:         env,
		Context:     map[string]string{"previous_verdict": core.VerdictFAIL},
	}
}

func TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace(t *testing.T) {
	ws := t.TempDir()
	projectRoot := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(projectRoot, ws, "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := fb.gotReq.Worktree
	if got == "" {
		t.Fatalf("BridgeRequest.Worktree is empty — the fleet guard refuses this launch and retro is lost for the whole lane")
	}
	if !strings.HasPrefix(got, ws+string(os.PathSeparator)) {
		t.Fatalf("BridgeRequest.Worktree = %q, want a directory under the owned workspace %q", got, ws)
	}
	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("fallback cwd %q is not an existing directory (err=%v) — the bridge rejects a non-existent working dir", got, err)
	}
}

func TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd(t *testing.T) {
	ws := t.TempDir()
	projectRoot := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(projectRoot, ws, "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := fb.gotReq.Worktree
	if got == "" {
		t.Fatalf("BridgeRequest.Worktree is empty — the launch is refused outright, so the safe-destination contract below is untested")
	}
	if got == projectRoot || strings.HasPrefix(got, projectRoot+string(os.PathSeparator)) {
		t.Fatalf("BridgeRequest.Worktree = %q resolves inside the shared main tree %q — refuted by PR #400: worktree is the write-authority predicate", got, projectRoot)
	}
	if cwd, err := os.Getwd(); err == nil && got == cwd {
		t.Fatalf("BridgeRequest.Worktree = %q is the dispatching process cwd — the exact leak the fleet guard closes", got)
	}
}

func TestRetro_RealWorktree_PassedThroughUnchanged(t *testing.T) {
	ws := t.TempDir()
	worktree := t.TempDir()
	fb := &fakeBridge{writeArtifact: "# Retrospective\n\n## Root Cause\nx\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(t.TempDir(), ws, worktree, map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fb.gotReq.Worktree != worktree {
		t.Fatalf("BridgeRequest.Worktree = %q, want the provisioned worktree %q left untouched", fb.gotReq.Worktree, worktree)
	}
	if _, err := os.Stat(filepath.Join(ws, "bridge-scratch-cwd")); err == nil {
		t.Errorf("a scratch cwd was minted under the workspace even though a real worktree was provisioned")
	}
}

func TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath(t *testing.T) {
	fb := &fakeBridge{}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})

	if _, err := phase.Run(context.Background(), retroFailReq(t.TempDir(), "", "", map[string]string{"EVOLVE_FLEET": "1"})); err != nil {
		t.Fatalf("Run returned a hard error with no workspace (%v) — retro must never abort the batch", err)
	}
	if got := fb.gotReq.Worktree; got != "" {
		t.Fatalf("BridgeRequest.Worktree = %q with no owned workspace — a fabricated path outside any owned dir is exactly the leak surface", got)
	}
}
