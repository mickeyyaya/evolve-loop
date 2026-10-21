package subagent

import (
	"context"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestExecAdapter_PassesTheProjectRootSoTheDispatchSweepCanRefuseThePlaneRoot(t *testing.T) {
	b := &fakeBridge{}
	prev := newAdapterBridge
	newAdapterBridge = func(gobridge.Deps) core.Bridge { return b }
	t.Cleanup(func() { newAdapterBridge = prev })

	_, err := execAdapter(context.Background(), map[string]string{"RESOLVED_CLI": "claude-tmux", "EVOLVE_PROJECT_ROOT": "/plane", "WORKTREE_PATH": "/plane"}, nil)

	if err != nil || len(b.calls) != 1 || b.calls[0].ProjectRoot != "/plane" || b.calls[0].Worktree != "/plane" {
		t.Fatalf("execAdapter err %v, calls %+v; the request must carry EVOLVE_PROJECT_ROOT as ProjectRoot", err, b.calls)
	}
}
