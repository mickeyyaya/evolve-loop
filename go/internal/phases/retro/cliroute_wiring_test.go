package retro

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func retroRouter(t *testing.T, root string, pol policy.Policy) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: pol, Profiles: profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func TestRetro_DispatchesTheResolverPrimary(t *testing.T) {
	root := t.TempDir()
	writeRetroProfile(t, root, "codex-tmux")
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	fb := &fakeBridge{resp: core.BridgeResponse{ExitCode: 0}, writeArtifact: "# retro\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body"), Router: retroRouter(t, root, policy.Policy{CLIRouting: &block})})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Workspace: t.TempDir(), ProjectRoot: root, Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
	})
	if fb.gotReq.CLI != "agy-tmux" {
		t.Fatalf("the retro launches the table's primary, not the profile's codex: %q", fb.gotReq.CLI)
	}
}

func TestRetro_ARoutingRefusalIsAFailVerdictWithTheReason(t *testing.T) {
	root := t.TempDir()
	writeRetroProfile(t, root, "codex-tmux")
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	fb := &fakeBridge{resp: core.BridgeResponse{ExitCode: 0}, writeArtifact: "# retro\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body"), Router: retroRouter(t, root, policy.Policy{CLIRouting: &block})})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Workspace: t.TempDir(), ProjectRoot: root, Cycle: 1,
		Context: map[string]string{"previous_verdict": core.VerdictFAIL},
		Env:     map[string]string{"EVOLVE_RETROSPECTIVE_CLI": "codex-tmux"},
	})
	if err != nil || resp.Verdict != core.VerdictFAIL || fb.gotReq.CLI != "" || !strings.Contains(resp.Diagnostics[len(resp.Diagnostics)-1].Message, "outside the allowed set") {
		t.Fatalf("a refused route fails the retro without launching: %+v %v launched=%q", resp, err, fb.gotReq.CLI)
	}
}
