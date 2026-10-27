package swarmplan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_ShouldSkip_APolicyMandatorySwarmPlanRuns(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"mandatory_phases":["swarm-plan"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if skip, _, _, _ := New(Config{}).ShouldSkip(core.PhaseRequest{ProjectRoot: root}); skip {
		t.Error("a swarm-plan in the policy mandatory_phases must run")
	}
}
