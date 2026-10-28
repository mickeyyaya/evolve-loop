//go:build integration

package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestAttemptRepair_OncePerCode(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	stalePinStateJSON(t, repo)

	opts := Options{
		Class:          ClassCycle,
		CommitMessage:  "guard",
		ProjectRoot:    repo,
		PluginRoot:     repo,
		ShipBinaryPath: filepath.Join(repo, "ship-binary-fixture"),
		Runner:         execRunner,
	}
	res := RunResult{}
	tampered := shipErr(core.CodeSelfSHATampered, core.ShipClassIntegrity, core.StageVerifySelfSHA, "stale pin")

	first := attemptRepair(context.Background(), &opts, &res, tampered)
	if first == repairNone {
		t.Fatalf("first attempt for a healable stale pin must run the repair, got repairNone")
	}
	second := attemptRepair(context.Background(), &opts, &res, tampered)
	if second != repairNone {
		t.Fatalf("second attempt for the SAME code must decline (once-guard), got %v", second)
	}
}
