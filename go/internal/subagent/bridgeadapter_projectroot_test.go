package subagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestExecAdapter_TheRetiredBareNameGeminiLaunchesNothing(t *testing.T) {
	b := &fakeBridge{}
	prev := newAdapterBridge
	newAdapterBridge = func(gobridge.Deps) core.Bridge { return b }
	t.Cleanup(func() { newAdapterBridge = prev })

	code, err := execAdapter(context.Background(), map[string]string{"RESOLVED_CLI": "gemini"}, nil)

	if err == nil || code != -1 || len(b.calls) != 0 || !strings.Contains(err.Error(), "agy-tmux") {
		t.Fatalf("execAdapter(gemini) = %d, %v with %d launches; the bare name must fail before any launch and name the agy drivers", code, err, len(b.calls))
	}
}

func TestExecAdapterDeps_CarryTheProjectFloors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["agy"],"default":["agy"],"floors":{"agy":["auditor"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := execAdapterDeps(map[string]string{"EVOLVE_PROJECT_ROOT": root}).Floors.FamilyOf("auditor"); got != "agy" {
		t.Fatalf("the bridge deps floor auditor on %q, want the project's agy floor", got)
	}
}
