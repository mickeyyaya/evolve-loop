package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestRun_MarksEveryAttemptAsAChainAttempt — the runner walks its own
// CLI/tier chain (DispatchTiered); every request it hands the bridge is one
// attempt of that walk and says so, so the chain-walking bridge handle the
// composition root installs passes it straight through instead of walking the
// chain a second time around it.
func TestRun_MarksEveryAttemptAsAChainAttempt(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "composed body", verdict: core.VerdictPASS, nextPhase: "audit"}
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-builder", "agent body"), NowFn: fixtures.FixedClock(time.Unix(1_700_000_000, 0), 200*time.Millisecond)})
	if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 9, ProjectRoot: t.TempDir(), Workspace: t.TempDir(), RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !fb.gotReq.ChainAttempt {
		t.Fatalf("the runner's attempt must carry ChainAttempt: %+v", fb.gotReq.CLI)
	}
}
