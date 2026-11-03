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

const fenceRevertedPatch = "fence-reverted.patch"

func runFencedWithBridge(t *testing.T, phase string, readOnly bool, bridge core.Bridge) (dir, workspace string, resp core.PhaseResponse) {
	t.Helper()
	dir = fenceRepo(t)
	workspace = t.TempDir()
	hooks := &fakeHooks{phase: phase, agent: "evolve-" + phase, model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-"+phase, "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: workspace, Worktree: dir, Fence: core.FenceFor(!readOnly, nil)}
	resp, err := r.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return dir, workspace, resp
}

func allFenceDiagnostics(resp core.PhaseResponse) string {
	var parts []string
	for _, d := range resp.Diagnostics {
		if strings.HasPrefix(d.Message, "worktree fence:") {
			parts = append(parts, d.Message)
		}
	}
	return strings.Join(parts, "\n")
}

func TestRun_FencedWriteLeavesAPatchInTheWorkspaceThatGivesItBack(t *testing.T) {
	t.Parallel()
	dir, workspace, _ := runFencedWithBridge(t, "audit", true, &mutatingBridge{})
	if got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go")); string(got) != "package src // builder change\n" {
		t.Fatalf("the fence did not restore the dispatched tree: %q", got)
	}
	patch := filepath.Join(workspace, fenceRevertedPatch)
	if _, err := os.Stat(patch); err != nil {
		t.Fatalf("a fenced write must leave %s in the request workspace: %v", patch, err)
	}
	cmd := exec.Command("git", "-C", dir, "apply", patch)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git apply of the saved patch on the dispatched tree: %v: %s", err, out)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go")); string(got) != "package src // MUTATED by a probe\n" {
		t.Errorf("patch did not give back the reverted content of src/mat.go: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "src", "zz_probe_test.go")); string(got) != "package src // probe\n" {
		t.Errorf("patch did not give back the reverted new file: %q", got)
	}
}

func TestRun_FenceDiagnosticNamesThePatchPath(t *testing.T) {
	t.Parallel()
	_, workspace, resp := runFencedWithBridge(t, "audit", true, &mutatingBridge{})
	want := filepath.Join(workspace, fenceRevertedPatch)
	if text := allFenceDiagnostics(resp); !strings.Contains(text, want) {
		t.Errorf("the fence diagnostic must name %s, got %q (diags=%+v)", want, text, resp.Diagnostics)
	}
}

func TestRun_NoFencedWriteWritesNoPatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		phase    string
		readOnly bool
		bridge   core.Bridge
	}{
		{"read-only phase that writes nothing", "audit", true, &fakeBridge{}},
		{"source writer is not fenced", "build", false, &mutatingBridge{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, workspace, resp := runFencedWithBridge(t, tc.phase, tc.readOnly, tc.bridge)
			if _, err := os.Stat(filepath.Join(workspace, fenceRevertedPatch)); !os.IsNotExist(err) {
				t.Errorf("no fenced write must mean no patch file: %v", err)
			}
			for _, d := range resp.Diagnostics {
				if strings.Contains(d.Message, fenceRevertedPatch) {
					t.Errorf("no fenced write must mean no patch named in a diagnostic: %q", d.Message)
				}
			}
		})
	}
}
