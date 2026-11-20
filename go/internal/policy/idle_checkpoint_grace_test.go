package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdleCheckpointGrace_LoadsFromTheBridgeBlock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"bridge":{"idle_checkpoint_grace_s":17}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pol, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pol.BridgeConfig().IdleCheckpointGraceS; got != 17 {
		t.Errorf("IdleCheckpointGraceS = %d, want 17 from bridge.idle_checkpoint_grace_s", got)
	}
}

func TestIdleCheckpointGrace_UnsetStaysZeroSoTheBridgeDefaultApplies(t *testing.T) {
	t.Parallel()
	if got := (Policy{}).BridgeConfig().IdleCheckpointGraceS; got != 0 {
		t.Errorf("IdleCheckpointGraceS = %d, want 0 when policy.json sets nothing", got)
	}
}
