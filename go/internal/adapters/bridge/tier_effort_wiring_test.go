package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductionDepsCarryThePolicyTierEffort(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"bridge":{"tier_effort":{"deep":"medium"}}}`), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root, nil).productionEngineDeps(map[string]string{"HOME": root})
	if deps.TierEffort["deep"] != "medium" || deps.TierEffort["top"] != "xhigh" {
		t.Errorf("TierEffort = %v, want deep=medium from policy and top=xhigh compiled: the bridge.tier_effort block never reaches the engine", deps.TierEffort)
	}
}
