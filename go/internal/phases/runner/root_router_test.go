package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func withNoRootRouter(t *testing.T) {
	t.Helper()
	orig := DefaultRouter
	t.Cleanup(func() { DefaultRouter = orig })
	DefaultRouter = nil
}

func writePolicy(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDispatchPlan_ADeclaredTableWithNoRootRouterFailsLoudly(t *testing.T) {
	withNoRootRouter(t)
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", []string{"claude-tmux"})
	writePolicy(t, root, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})

	_, resp, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))

	if err == nil || resp == nil || resp.Verdict != core.VerdictFAIL || !strings.Contains(resp.Diagnostics[0].Message, "no compiled router") {
		t.Fatalf("a runner with no root router must not route a declared table on its own: %v %+v", err, resp)
	}
}

func TestResolveDispatchPlan_NoTableAndNoRootRouterKeepsTheProfileRoute(t *testing.T) {
	withNoRootRouter(t)
	root := writeFallbackProfile(t, "evolve-scout", "codex-tmux", []string{"claude-tmux"})
	writePolicy(t, root, `{"workflow":{"universal_fallback_exclude":[]}}`)
	b := New(Options{Hooks: &fakeHooks{phase: "scout", agent: "evolve-scout"}, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-scout", "x")})

	resolved, resp, err := b.resolveDispatchPlan(core.PhaseRequest{ProjectRoot: root}, scoutPreparation(root))

	if err != nil || resp != nil || resolved.plan.Candidates[0] != "codex-tmux" {
		t.Fatalf("a legacy project routes per launch from the profile as before: %+v %v %+v", resolved.plan, err, resp)
	}
}
